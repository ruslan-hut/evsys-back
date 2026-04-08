package entity

import (
	"evsys-back/internal/lib/validate"
	"net/http"
	"time"
)

type PaymentOrder struct {
	TransactionId int       `json:"transaction_id" bson:"transaction_id" validate:"min=0"`
	Order         int       `json:"order" bson:"order" validate:"min=0"`
	UserId        string    `json:"user_id" bson:"user_id" validate:"omitempty"`
	UserName      string    `json:"user_name" bson:"user_name" validate:"omitempty"`
	Amount        int       `json:"amount" bson:"amount" validate:"min=0"`
	Currency      string    `json:"currency" bson:"currency" validate:"required,len=3"`
	Description   string    `json:"description" bson:"description" validate:"omitempty"`
	Identifier    string    `json:"identifier" bson:"identifier" validate:"omitempty"`
	IsCompleted   bool      `json:"is_completed" bson:"is_completed"`
	Result        string    `json:"result" bson:"result" validate:"omitempty"`
	Date          string    `json:"date" bson:"date" validate:"omitempty"`
	TimeOpened    time.Time `json:"time_opened" bson:"time_opened"`
	TimeClosed    time.Time `json:"time_closed" bson:"time_closed"`
	RefundAmount  int       `json:"refund_amount" bson:"refund_amount" validate:"min=0"`
	RefundTime    time.Time `json:"refund_time" bson:"refund_time"`
	// Mode selects the response variant: empty (default) for the Android/native
	// SDK flow, "web" for the Redsys TPV Virtual hosted-form redirect flow
	// used by the "add card" page in the Angular app. Not persisted.
	Mode string `json:"mode,omitempty" bson:"-" validate:"omitempty,oneof=web"`

	// ReturnUrlOk / ReturnUrlKo are the browser-facing redirect targets
	// used by the web flow. Not persisted. Ignored for the legacy mode.
	ReturnUrlOk string `json:"return_url_ok,omitempty" bson:"-" validate:"omitempty,url"`
	ReturnUrlKo string `json:"return_url_ko,omitempty" bson:"-" validate:"omitempty,url"`
	// Language is a Redsys numeric language code, e.g. "001" Spanish,
	// "002" English. Optional — the client leaves it blank to fall back
	// to the backend default.
	Language string `json:"language,omitempty" bson:"-" validate:"omitempty"`
}

func (p *PaymentOrder) Bind(_ *http.Request) error {
	return validate.Struct(p)
}

// WebOrderResponse is returned by POST /payment/order when mode=="web".
// It embeds the stored PaymentOrder (so callers still see the generated
// order number and timestamps) and adds the signed Redsys TPV Virtual
// hosted-form triplet the browser must auto-POST to `form_url`. On
// success, Redsys calls POST /payment/notify server-to-server with the
// permanent card token and then redirects the browser to return_url_ok.
type WebOrderResponse struct {
	*PaymentOrder
	FormUrl            string `json:"form_url"`
	SignatureVersion   string `json:"Ds_SignatureVersion"`
	MerchantParameters string `json:"Ds_MerchantParameters"`
	Signature          string `json:"Ds_Signature"`
}
