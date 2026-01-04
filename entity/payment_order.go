package entity

import (
	"evsys-back/internal/lib/validate"
	"net/http"
	"time"
)

type PaymentOrder struct {
	TransactionId int       `json:"transaction_id" bson:"transaction_id" validate:"required,min=1"`
	Order         int       `json:"order" bson:"order" validate:"required,min=1"`
	UserId        string    `json:"user_id" bson:"user_id" validate:"required"`
	UserName      string    `json:"user_name" bson:"user_name" validate:"omitempty"`
	Amount        int       `json:"amount" bson:"amount" validate:"required,min=0"`
	Currency      string    `json:"currency" bson:"currency" validate:"required,len=3"`
	Description   string    `json:"description" bson:"description" validate:"omitempty"`
	Identifier    string    `json:"identifier" bson:"identifier" validate:"required"`
	IsCompleted   bool      `json:"is_completed" bson:"is_completed"`
	Result        string    `json:"result" bson:"result" validate:"omitempty"`
	Date          string    `json:"date" bson:"date" validate:"omitempty"`
	TimeOpened    time.Time `json:"time_opened" bson:"time_opened"`
	TimeClosed    time.Time `json:"time_closed" bson:"time_closed"`
	RefundAmount  int       `json:"refund_amount" bson:"refund_amount" validate:"min=0"`
	RefundTime    time.Time `json:"refund_time" bson:"refund_time"`
}

func (p *PaymentOrder) Bind(_ *http.Request) error {
	return validate.Struct(p)
}
