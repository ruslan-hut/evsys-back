package entity

import "time"

// WebhookOutboxStats is the per-subscriber aggregation over webhook_outbox.
type WebhookOutboxStats struct {
	Subscriber    string     `json:"subscriber" bson:"_id"`
	Pending       int        `json:"pending" bson:"pending"`
	Delivered     int        `json:"delivered" bson:"delivered"`
	Failed        int        `json:"failed" bson:"failed"`
	OldestPending *time.Time `json:"oldest_pending,omitempty" bson:"oldest_pending,omitempty"`
	LastDelivered *time.Time `json:"last_delivered,omitempty" bson:"last_delivered,omitempty"`
}

// WebhookHealthView combines a subscriber's configuration with its delivery
// counters for the admin UI. A row without config (Configured=false) means
// outbox documents exist for a subscriber that has since been deleted.
type WebhookHealthView struct {
	Name          string     `json:"name"`
	URL           string     `json:"url,omitempty"`
	IsEnabled     bool       `json:"is_enabled"`
	Configured    bool       `json:"configured"`
	Pending       int        `json:"pending"`
	Delivered     int        `json:"delivered"`
	Failed        int        `json:"failed"`
	OldestPending *time.Time `json:"oldest_pending,omitempty"`
	LastDelivered *time.Time `json:"last_delivered,omitempty"`
}

// WebhookDeliveryView is one outbox document as shown in the admin failure list;
// the payload bytes are deliberately not exposed.
type WebhookDeliveryView struct {
	EventId     string    `json:"event_id" bson:"event_id"`
	Subscriber  string    `json:"subscriber" bson:"subscriber"`
	Type        string    `json:"type" bson:"type"`
	Sequence    int64     `json:"sequence" bson:"sequence"`
	Status      string    `json:"status" bson:"status"`
	Attempts    int       `json:"attempts" bson:"attempts"`
	NextAttempt time.Time `json:"next_attempt" bson:"next_attempt"`
	LastError   string    `json:"last_error,omitempty" bson:"last_error"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
}
