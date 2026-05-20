package entity

import "time"

type PaymentRetry struct {
	TransactionId int       `json:"transaction_id" bson:"transaction_id"`
	Attempt       int       `json:"attempt" bson:"attempt"`
	NextRetryTime time.Time `json:"next_retry_time" bson:"next_retry_time"`
	LastError     string    `json:"last_error" bson:"last_error"`
	CreatedAt     time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" bson:"updated_at"`
}

// PaymentRetryView is a read-only retry-queue entry enriched with transaction
// summary fields for the admin preview.
type PaymentRetryView struct {
	TransactionId int        `json:"transaction_id"`
	Attempt       int        `json:"attempt"`
	MaxAttempts   int        `json:"max_attempts"`
	NextRetryTime time.Time  `json:"next_retry_time"`
	LastError     string     `json:"last_error"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ChargePointId string     `json:"charge_point_id,omitempty"`
	Username      string     `json:"username,omitempty"`
	PaymentAmount int        `json:"payment_amount,omitempty"`
	TimeStart     *time.Time `json:"time_start,omitempty"`
}
