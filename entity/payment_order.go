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
	// SDK flow, "insite" for the Redsys inSite web flow. Not persisted.
	Mode string `json:"mode,omitempty" bson:"-" validate:"omitempty,oneof=insite"`
}

func (p *PaymentOrder) Bind(_ *http.Request) error {
	return validate.Struct(p)
}

// InSiteOrderResponse is the response returned by POST /payment/order when
// mode=="insite". It embeds the stored PaymentOrder (so callers still see
// the generated order number and timestamps) and exposes the three merchant
// identifiers the Redsys inSite JS SDK needs to draw the card form:
//
//	getInSiteFormJSON({ fuc, terminal, order, ... })
//
// No signing happens at this stage — inSite does not require it. The
// signed REST call to Redsys happens later, on /payment/tokenize, after
// the browser has handed us the temporary idOper.
type InSiteOrderResponse struct {
	*PaymentOrder
	MerchantCode string `json:"merchant_code"`
	Terminal     string `json:"terminal"`
	OrderNumber  string `json:"order_number"`
}

// TokenizeRequest is the payload sent by the web client to
// POST /payment/tokenize after the Redsys inSite SDK has issued a
// temporary operation identifier (idOper, valid for 30 minutes).
type TokenizeRequest struct {
	Order       int    `json:"order" validate:"required,min=1"`
	IdOper      string `json:"id_oper" validate:"required"`
	Description string `json:"description" validate:"omitempty"`
}

func (r *TokenizeRequest) Bind(_ *http.Request) error {
	return validate.Struct(r)
}
