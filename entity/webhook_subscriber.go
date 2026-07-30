package entity

import (
	"evsys-back/internal/lib/validate"
	"net/http"
	"time"
)

// WebhookSubscriber mirrors the document evsys reads from the webhook_subscribers
// collection; name, url, token, events, is_enabled and updated_at are the contract
// with evsys and must keep their bson tags (see evsys entity/webhook_subscriber.go
// and docs/WEBHOOKS.md there).
type WebhookSubscriber struct {
	Id        string    `json:"id" bson:"_id,omitempty"`
	Name      string    `json:"name" bson:"name" validate:"required"`
	URL       string    `json:"url" bson:"url" validate:"required,url"`
	Token     string    `json:"token" bson:"token"`
	Events    []string  `json:"events" bson:"events" validate:"required,min=1,dive,required"`
	IsEnabled bool      `json:"is_enabled" bson:"is_enabled"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

func (s *WebhookSubscriber) Bind(_ *http.Request) error {
	return validate.Struct(s)
}
