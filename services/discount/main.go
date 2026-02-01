package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/devdolphintest/discount-system/pkg/common"
	"github.com/devdolphintest/discount-system/pkg/events"
	"github.com/joho/godotenv"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	ProjectID        = "devdolphins-93118"
	CollectionEvents = "events"
	CollectionQuotas = "daily_quotas"
	QuotaLimit       = 100 // R1
	ISTOffset        = 5*time.Hour + 30*time.Minute
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

func main() {
	_ = godotenv.Load()
	ctx := context.Background()
	client, err := common.NewFirestoreClient(ctx, ProjectID)
	if err != nil {
		logger.Error("Failed to create client", "error", err)
		os.Exit(1)
	}
	defer client.Close()

	logger.Info("Discount Service Started", "limit", QuotaLimit)

	// Listen for OrderCreated and DiscountRelease events
	iter := client.Collection(CollectionEvents).
		Where("type", "in", []string{events.EventTypeOrderCreated, events.EventTypeDiscountRelease}).
		OrderBy("timestamp", firestore.Asc).
		Snapshots(ctx)
	defer iter.Stop()

	for {
		snap, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			logger.Error("Error listening to events", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		for _, change := range snap.Changes {
			if change.Kind == firestore.DocumentAdded {
				eventType := change.Doc.Data()["type"].(string)
				switch eventType {
				case events.EventTypeOrderCreated:
					processOrderEvent(ctx, client, change.Doc)
				case events.EventTypeDiscountRelease:
					processReleaseEvent(ctx, client, change.Doc)
				}
			}
		}
	}
}

func processOrderEvent(ctx context.Context, client *firestore.Client, doc *firestore.DocumentSnapshot) {
	var event events.OrderCreated
	if err := doc.DataTo(&event); err != nil {
		logger.Error("Failed to parse event", "id", doc.Ref.ID, "error", err)
		return
	}

	// Only process R1-eligible requests
	if !event.IsR1Eligible {
		logger.Info("Skipping Non-R1-Eligible Order", "order_id", event.OrderID, "trace_id", event.TraceID)
		return
	}

	logger.Info("Processing R1-Eligible Order", "order_id", event.OrderID, "trace_id", event.TraceID,
		"base_price", event.BasePrice, "discount", event.DiscountPercent)

	// Idempotency Check: Don't process if we already reacted
	exists, err := checkDecisionExists(ctx, client, event.OrderID)
	if err != nil {
		logger.Error("Failed to check existing decision", "trace_id", event.TraceID, "error", err)
		return
	}
	if exists {
		logger.Info("Decision Already Exists", "order_id", event.OrderID, "trace_id", event.TraceID)
		return
	}

	if err := runQuotaTransaction(ctx, client, event); err != nil {
		logger.Error("Transaction failed", "trace_id", event.TraceID, "error", err)
	}
}

func checkDecisionExists(ctx context.Context, client *firestore.Client, orderID string) (bool, error) {
	// Note: 'IN' query might verify indices. Let's do two checks or just one if we can.
	// Firestore supports 'In' operator now.
	q := client.Collection(CollectionEvents).
		Where("order_id", "==", orderID).
		Where("type", "in", []string{events.EventTypeDiscountReserved, events.EventTypeDiscountRejected}).
		Limit(1)

	snaps, err := q.Documents(ctx).GetAll()
	if err != nil {
		return false, err
	}
	return len(snaps) > 0, nil
}

func runQuotaTransaction(ctx context.Context, client *firestore.Client, event events.OrderCreated) error {
	return client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 1. Determine Date in IST
		ist := time.FixedZone("IST", int(ISTOffset.Seconds()))
		today := time.Now().In(ist).Format("2006-01-02")
		quotaRef := client.Collection(CollectionQuotas).Doc(today)

		// 2. Read current quota
		// Note: Document might not exist yet.
		doc, err := tx.Get(quotaRef)
		currentCount := int64(0)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				// It's a new day, count is 0
			} else {
				return err
			}
		} else {
			if v, ok := doc.Data()["count"].(int64); ok {
				currentCount = v
			}
		}

		// 3. Decision
		var decisionEvent interface{}

		if currentCount < QuotaLimit {
			// Approve
			newCount := currentCount + 1
			if err := tx.Set(quotaRef, map[string]interface{}{"count": newCount}, firestore.MergeAll); err != nil {
				return err
			}

			decisionEvent = events.DiscountReserved{
				BaseEvent: events.BaseEvent{
					TraceID:   event.TraceID,
					Type:      events.EventTypeDiscountReserved,
					Timestamp: time.Now(),
				},
				OrderID: event.OrderID,
				Status:  "Approved",
			}
			logger.Info("R2 Quota Reserved", "trace_id", event.TraceID, "order_id", event.OrderID,
				"quota_used", newCount, "quota_remaining", QuotaLimit-newCount)
		} else {
			// Reject
			decisionEvent = events.DiscountRejected{
				BaseEvent: events.BaseEvent{
					TraceID:   event.TraceID,
					Type:      events.EventTypeDiscountRejected,
					Timestamp: time.Now(),
				},
				OrderID: event.OrderID,
				Status:  "Rejected",
				Reason:  "Daily discount quota reached. Please try again tomorrow.",
			}
			logger.Info("R2 Quota Exhausted", "trace_id", event.TraceID, "order_id", event.OrderID,
				"quota_limit", QuotaLimit, "current_count", currentCount)
		}

		// 4. Publish Decision
		// We use a new document for the event.
		newEventRef := client.Collection(CollectionEvents).NewDoc()
		return tx.Set(newEventRef, decisionEvent)
	})
}

func processReleaseEvent(ctx context.Context, client *firestore.Client, doc *firestore.DocumentSnapshot) {
	var event events.DiscountRelease
	if err := doc.DataTo(&event); err != nil {
		logger.Error("Failed to parse release event", "id", doc.Ref.ID, "error", err)
		return
	}

	// Check if we need to release for this OrderID
	// Idempotency: Ideally track "Released" state.
	// For now, transactionally decrement.

	err := client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// Find if we approved this order.
		// We can query events to see if we approved it.
		// OR simply decrement today's quota.
		// Using "Date from Timestamp" of the ORIGINAL order would be best, but we might just use today or event timestamp.
		// Let's assume release happens same day or we accept slight skew for simplicity.
		// Better: Store "Reserved" status in a separate collection 'reservations' to track Date.
		// For this demo: use Today's quota.

		ist := time.FixedZone("IST", int(ISTOffset.Seconds()))
		today := time.Now().In(ist).Format("2006-01-02")
		quotaRef := client.Collection(CollectionQuotas).Doc(today)

		doc, err := tx.Get(quotaRef)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				// Quota document for today doesn't exist, meaning no quotas were used today.
				// Nothing to decrement.
				logger.Info("Quota document not found for today, nothing to release", "order_id", event.OrderID, "date", today)
				return nil // Successfully did nothing
			}
			return err
		}

		count := doc.Data()["count"].(int64)
		if count > 0 {
			if err := tx.Set(quotaRef, map[string]interface{}{"count": count - 1}, firestore.MergeAll); err != nil {
				return err
			}
			logger.Info("Quota Compensation Executed", "order_id", event.OrderID, "new_count", count-1)
		} else {
			logger.Info("Quota count is already zero, nothing to decrement", "order_id", event.OrderID, "date", today)
		}
		return nil
	})

	if err != nil {
		logger.Error("Compensation failed", "order_id", event.OrderID, "error", err)
	}
}

// statusCode extracts gRPC status code, simple helper needed because err isn't directly grpc error always
// Simplified for this context: just assume standard error checks or use error strings if needed.
// Actually firestore.NotFound is simpler.
func statusCode(err error) int {
	// This is a dummy implementation. In real code we'd use status.Code(err)
	// But client.Get returns specific error for not found.
	// Let's stick to checking string or use firestore wrapper.
	// Rewriting usage above to just use Error check logic provided by client lib patterns.
	return 0
}
