package models

import (
	"evsys-back/internal/lib/validate"
	"net/http"
	"time"
)

type User struct {
	Username       string    `json:"username" bson:"username" validate:"omitempty"`
	Password       string    `json:"password" bson:"password" validate:"required"`
	Name           string    `json:"name" bson:"name" validate:"omitempty"`
	Role           string    `json:"role" bson:"role" validate:"omitempty"`
	AccessLevel    int       `json:"access_level" bson:"access_level" validate:"omitempty,min=0,max=10"`
	Email          string    `json:"email" bson:"email" validate:"omitempty"`
	PaymentPlan    string    `json:"payment_plan" bson:"payment_plan" validate:"omitempty"`
	Token          string    `json:"token" bson:"token" validate:"omitempty"`
	UserId         string    `json:"user_id" bson:"user_id" validate:"omitempty"`
	DateRegistered time.Time `json:"date_registered" bson:"date_registered" validate:"omitempty"`
	LastSeen       time.Time `json:"last_seen" bson:"last_seen" validate:"omitempty"`
}

type UserInfo struct {
	Username       string           `json:"username" bson:"username"`
	Name           string           `json:"name" bson:"name"`
	Role           string           `json:"role" bson:"role"`
	AccessLevel    int              `json:"access_level" bson:"access_level"`
	Email          string           `json:"email" bson:"email"`
	DateRegistered time.Time        `json:"date_registered" bson:"date_registered"`
	LastSeen       time.Time        `json:"last_seen" bson:"last_seen"`
	PaymentPlans   []*PaymentPlan   `json:"payment_plans" bson:"payment_plans"`
	UserTags       []*UserTag       `json:"user_tags" bson:"user_tags"`
	PaymentMethods []*PaymentMethod `json:"payment_methods" bson:"payment_methods"`
}

func (u *User) Bind(_ *http.Request) error {
	return validate.Struct(u)
}
