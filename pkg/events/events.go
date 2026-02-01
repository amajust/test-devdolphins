package events

import (
	"time"
)

// EventType constants
const (
	EventTypeOrderCreated     = "OrderCreated"
	EventTypeDiscountReserved = "DiscountReserved"
	EventTypeDiscountRejected = "DiscountRejected"
	EventTypeDiscountRelease  = "DiscountRelease"
)

// BaseEvent contains common fields for all events
type BaseEvent struct {
	TraceID   string    `json:"trace_id" firestore:"trace_id"`
	Type      string    `json:"type" firestore:"type"`
	Timestamp time.Time `json:"timestamp" firestore:"timestamp"`
}

// Service represents a medical service
type Service struct {
	Name  string  `json:"name" firestore:"name"`
	Price float64 `json:"price" firestore:"price"`
}

// OrderCreated represents a new order request
type OrderCreated struct {
	BaseEvent
	OrderID          string    `json:"order_id" firestore:"order_id"`
	UserID           string    `json:"user_id" firestore:"user_id"`
	Name             string    `json:"name" firestore:"name"`
	Gender           string    `json:"gender" firestore:"gender"`
	DOB              string    `json:"dob" firestore:"dob"`
	SelectedServices []Service `json:"selected_services" firestore:"selected_services"`
	BasePrice        float64   `json:"base_price" firestore:"base_price"`
	IsR1Eligible     bool      `json:"is_r1_eligible" firestore:"is_r1_eligible"`
	DiscountPercent  float64   `json:"discount_percent" firestore:"discount_percent"`
	FinalPrice       float64   `json:"final_price" firestore:"final_price"`
}

// DiscountReserved represents a successful discount reservation
type DiscountReserved struct {
	BaseEvent
	OrderID string `json:"order_id" firestore:"order_id"`
	Status  string `json:"status" firestore:"status"` // "Approved"
}

// DiscountRejected represents a failed discount reservation (quota full)
type DiscountRejected struct {
	BaseEvent
	OrderID string `json:"order_id" firestore:"order_id"`
	Status  string `json:"status" firestore:"status"` // "Rejected"
	Reason  string `json:"reason" firestore:"reason"`
}

// DiscountRelease represents a compensation action to release a quota
type DiscountRelease struct {
	BaseEvent
	OrderID string `json:"order_id" firestore:"order_id"`
	Reason  string `json:"reason" firestore:"reason"`
}
