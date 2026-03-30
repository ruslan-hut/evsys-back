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
