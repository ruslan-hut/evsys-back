package entity

import (
	"evsys-back/internal/lib/validate"
	"net/http"
	"time"
)

const (
	MailPeriodDaily   = "daily"
	MailPeriodWeekly  = "weekly"
	MailPeriodMonthly = "monthly"
)

type MailSubscription struct {
	Id        string    `json:"id" bson:"_id,omitempty"`
	Email     string    `json:"email" bson:"email" validate:"required,email_rfc"`
	Period    string    `json:"period" bson:"period" validate:"required,oneof=daily weekly monthly"`
	UserGroup string    `json:"user_group" bson:"user_group" validate:"required"`
	Enabled   bool      `json:"enabled" bson:"enabled"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

func (m *MailSubscription) Bind(_ *http.Request) error {
	return validate.Struct(m)
}
