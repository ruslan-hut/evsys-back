package entity

import (
	"evsys-back/internal/lib/validate"
	"net/http"
	"time"
)

type UserTag struct {
	Username       string    `json:"username" bson:"username" validate:"required"`
	UserId         string    `json:"user_id" bson:"user_id" validate:"omitempty"`
	IdTag          string    `json:"id_tag" bson:"id_tag" validate:"required"`
	Source         string    `json:"source" bson:"source" validate:"omitempty"`
	IsEnabled      bool      `json:"is_enabled" bson:"is_enabled"`
	Local          bool      `json:"local" bson:"local"`
	Note           string    `json:"note" bson:"note" validate:"omitempty"`
	DateRegistered time.Time `json:"date_registered" bson:"date_registered"`
	LastSeen       time.Time `json:"last_seen" bson:"last_seen"`
}

// UserTagCreate is used for create requests
type UserTagCreate struct {
	Username  string `json:"username" validate:"required"`
	UserId    string `json:"user_id" validate:"omitempty"`
	IdTag     string `json:"id_tag" validate:"required"`
	Source    string `json:"source" validate:"omitempty"`
	IsEnabled *bool  `json:"is_enabled" validate:"omitempty"`
	Local     *bool  `json:"local" validate:"omitempty"`
	Note      string `json:"note" validate:"omitempty"`
}

func (u *UserTagCreate) Bind(_ *http.Request) error {
	return validate.Struct(u)
}

// UserTagUpdate is used for update requests where all fields are optional
type UserTagUpdate struct {
	Username  string `json:"username" validate:"omitempty"`
	Source    string `json:"source" validate:"omitempty"`
	IsEnabled *bool  `json:"is_enabled" validate:"omitempty"`
	Local     *bool  `json:"local" validate:"omitempty"`
	Note      string `json:"note" validate:"omitempty"`
}

func (u *UserTagUpdate) Bind(_ *http.Request) error {
	return validate.Struct(u)
}
