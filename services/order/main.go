package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/devdolphintest/discount-system/pkg/common"
	"github.com/devdolphintest/discount-system/pkg/events"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"google.golang.org/api/iterator"
)

const (
	ProjectID        = "devdolphins-93118"
	CollectionEvents = "events"
)

var (
	logger      = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	client      *firestore.Client
	responseMap = make(map[string]chan interface{})
	mapMutex    sync.RWMutex
)

type Service struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type OrderRequest struct {
	UserID           string    `json:"user_id"`
	Name             string    `json:"name"`
	Gender           string    `json:"gender"`
	DOB              string    `json:"dob"`
	SelectedServices []Service `json:"selected_services"`
	BasePrice        float64   `json:"base_price"`
	IsR1Eligible     bool      `json:"is_r1_eligible"`
	DiscountPercent  float64   `json:"discount_percent"`
	FinalPrice       float64   `json:"final_price"`
	SimulateFailure  bool      `json:"simulate_failure"`
}

type OrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func main() {
	_ = godotenv.Load()

	ctx := context.Background()
	var err error
	client, err = common.NewFirestoreClient(ctx, ProjectID)
	if err != nil {
		logger.Error("Failed to create client", "error", err)
		os.Exit(1)
	}
	defer client.Close()

	// Start Background Listener
	go listenForDecisions(ctx)

	http.HandleFunc("/order", handleOrder)
	logger.Info("Order Service listening on :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		logger.Error("Server failed", "error", err)
	}
}

func handleOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Body", http.StatusBadRequest)
		return
	}

	orderID := uuid.New().String()
	traceID := uuid.New().String()

	logger.Info("Order Received", "order_id", orderID, "trace_id", traceID, "user", req.Name,
		"base_price", req.BasePrice, "r1_eligible", req.IsR1Eligible, "final_price", req.FinalPrice)

	// If R1 not eligible, complete order immediately without quota check
	if !req.IsR1Eligible {
		if req.SimulateFailure {
			logger.Warn("Simulating Payment Failure (Non-Discount Order)", "order_id", orderID, "trace_id", traceID)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(OrderResponse{
				OrderID: orderID,
				Status:  "FAILED",
				Message: "Payment processing failed (simulated).",
			})
			return
		}

		logger.Info("Order Completed Without Discount", "order_id", orderID, "trace_id", traceID)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(OrderResponse{
			OrderID: orderID,
			Status:  "CONFIRMED",
			Message: fmt.Sprintf("Booking confirmed! Total: ₹%.2f (No discount applied)", req.FinalPrice),
		})
		return
	}

	// Setup Response Channel for R1-eligible requests
	respChan := make(chan interface{}, 1)
	mapMutex.Lock()
	responseMap[orderID] = respChan
	mapMutex.Unlock()

	defer func() {
		mapMutex.Lock()
		delete(responseMap, orderID)
		mapMutex.Unlock()
	}()

	// Publish OrderCreated event for discount quota check
	event := events.OrderCreated{
		BaseEvent: events.BaseEvent{
			TraceID:   traceID,
			Type:      events.EventTypeOrderCreated,
			Timestamp: time.Now(),
		},
		OrderID:          orderID,
		UserID:           req.UserID,
		Name:             req.Name,
		Gender:           req.Gender,
		DOB:              req.DOB,
		SelectedServices: convertToEventServices(req.SelectedServices),
		BasePrice:        req.BasePrice,
		IsR1Eligible:     req.IsR1Eligible,
		DiscountPercent:  req.DiscountPercent,
		FinalPrice:       req.FinalPrice,
	}

	_, _, err := client.Collection(CollectionEvents).Add(r.Context(), event)
	if err != nil {
		logger.Error("Failed to publish event", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	logger.Info("Order Event Published - Checking R2 Quota", "order_id", orderID, "trace_id", traceID)

	// Wait for response
	select {
	case decisionRaw := <-respChan:
		// Process Decision
		switch d := decisionRaw.(type) {
		case events.DiscountReserved:
			logger.Info("Discount Reserved", "order_id", orderID, "trace_id", traceID)

			if req.SimulateFailure {
				// Chaos Test: Simulate post-reservation failure
				logger.Warn("Simulating Failure after Reservation", "order_id", orderID, "trace_id", traceID)

				// Publish Compensation
				compEvent := events.DiscountRelease{
					BaseEvent: events.BaseEvent{
						TraceID:   traceID,
						Type:      events.EventTypeDiscountRelease,
						Timestamp: time.Now(),
					},
					OrderID: orderID,
					Reason:  "Payment Processing Failed (Simulated Failure)",
				}
				client.Collection(CollectionEvents).Add(context.Background(), compEvent)

				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(OrderResponse{
					OrderID: orderID,
					Status:  "FAILED",
					Message: "Payment processing failed. Discount quota has been released.",
				})
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(OrderResponse{
				OrderID: orderID,
				Status:  "CONFIRMED",
				Message: fmt.Sprintf("Booking confirmed! Final price: ₹%.2f (12%% discount applied)", req.FinalPrice),
			})

		case events.DiscountRejected:
			logger.Info("Discount Rejected", "order_id", orderID, "trace_id", traceID, "reason", d.Reason)
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(OrderResponse{
				OrderID: orderID,
				Status:  "REJECTED",
				Message: d.Reason,
			})
		}

	case <-time.After(10 * time.Second):
		logger.Error("Timeout waiting for discount decision", "order_id", orderID, "trace_id", traceID)
		http.Error(w, "Timeout waiting for discount service", http.StatusGatewayTimeout)
	}
}

func convertToEventServices(services []Service) []events.Service {
	result := make([]events.Service, len(services))
	for i, s := range services {
		result[i] = events.Service{Name: s.Name, Price: s.Price}
	}
	return result
}

func listenForDecisions(ctx context.Context) {
	iter := client.Collection(CollectionEvents).
		Where("type", "in", []string{events.EventTypeDiscountReserved, events.EventTypeDiscountRejected}).
		OrderBy("timestamp", firestore.Asc).
		Snapshots(ctx)
	defer iter.Stop()

	for {
		snap, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			logger.Error("Listener error", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		for _, change := range snap.Changes {
			if change.Kind == firestore.DocumentAdded {
				data := change.Doc.Data()
				eventType := data["type"].(string)
				orderID := data["order_id"].(string)

				mapMutex.RLock()
				ch, exists := responseMap[orderID]
				mapMutex.RUnlock()

				if exists {
					// Route to handler
					if eventType == events.EventTypeDiscountReserved {
						var e events.DiscountReserved
						change.Doc.DataTo(&e)
						ch <- e
					} else {
						var e events.DiscountRejected
						change.Doc.DataTo(&e)
						ch <- e
					}
				}
			}
		}
	}
}
