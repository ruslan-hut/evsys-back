package entity

import (
	"evsys-back/internal/lib/validate"
	"net/http"
)

type PaymentMethod struct {
	Description string `json:"description" bson:"description" validate:"omitempty"`
	Identifier  string `json:"identifier" bson:"identifier" validate:"required"`
	CardNumber  string `json:"card_number,omitempty" bson:"card_number" validate:"omitempty"`
	CardType    string `json:"card_type,omitempty" bson:"card_type" validate:"omitempty"`
	CardBrand   string `json:"card_brand,omitempty" bson:"card_brand" validate:"omitempty"`
	CardCountry string `json:"card_country,omitempty" bson:"card_country" validate:"omitempty"`
	ExpiryDate  string `json:"expiry_date,omitempty" bson:"expiry_date" validate:"omitempty"`
	IsDefault   bool   `json:"is_default" bson:"is_default"`
	UserId      string `json:"user_id" bson:"user_id" validate:"omitempty"`
	UserName    string `json:"user_name" bson:"user_name" validate:"omitempty"`
	FailCount   int    `json:"fail_count" bson:"fail_count" validate:"omitempty"`
}

func (p *PaymentMethod) Bind(_ *http.Request) error {
	return validate.Struct(p)
}
