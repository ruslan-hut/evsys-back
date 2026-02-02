package entity

import (
	"evsys-back/internal/lib/validate"
	"net/http"
	"time"
)

// PreauthorizationStatus represents the status of a preauthorization
type PreauthorizationStatus string

const (
	PreauthorizationStatusPending    PreauthorizationStatus = "PENDING"
	PreauthorizationStatusAuthorized PreauthorizationStatus = "AUTHORIZED"
	PreauthorizationStatusCaptured   PreauthorizationStatus = "CAPTURED"
	PreauthorizationStatusCancelled  PreauthorizationStatus = "CANCELLED"
	PreauthorizationStatusFailed     PreauthorizationStatus = "FAILED"
)

// Preauthorization represents a preauthorization record for EV charging payments
type Preauthorization struct {
	OrderNumber         string                 `json:"order_number" bson:"order_number"`
	AuthorizationCode   string                 `json:"authorization_code" bson:"authorization_code"`
	PreauthorizedAmount int                    `json:"preauthorized_amount" bson:"preauthorized_amount"`
	CapturedAmount      int                    `json:"captured_amount" bson:"captured_amount"`
	Status              PreauthorizationStatus `json:"status" bson:"status"`
	TransactionId       int                    `json:"transaction_id" bson:"transaction_id"`
	PaymentMethodId     string                 `json:"payment_method_id" bson:"payment_method_id"`
	UserId              string                 `json:"user_id" bson:"user_id"`
	UserName            string                 `json:"user_name" bson:"user_name"`
	Currency            string                 `json:"currency" bson:"currency"`
	MerchantData        string                 `json:"merchant_data,omitempty" bson:"merchant_data,omitempty"`
	ErrorCode           string                 `json:"error_code,omitempty" bson:"error_code,omitempty"`
	ErrorMessage        string                 `json:"error_message,omitempty" bson:"error_message,omitempty"`
	CreatedAt           time.Time              `json:"created_at" bson:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at" bson:"updated_at"`
}

// PreauthorizationOrderRequest is the request body for creating a preauthorization order
type PreauthorizationOrderRequest struct {
	TransactionId   int    `json:"transaction_id" validate:"required,min=1"`
	Amount          int    `json:"amount" validate:"required,min=1"`
	Currency        string `json:"currency" validate:"required,len=3"`
	PaymentMethodId string `json:"payment_method_id" validate:"required"`
	Description     string `json:"description" validate:"omitempty"`
}

func (r *PreauthorizationOrderRequest) Bind(_ *http.Request) error {
	return validate.Struct(r)
}

// PreauthorizationOrderResponse is the response for preauthorization order creation
type PreauthorizationOrderResponse struct {
	OrderNumber    string `json:"order_number"`
	Amount         int    `json:"amount"`
	Currency       string `json:"currency"`
	MerchantParams string `json:"merchant_params,omitempty"`
	Signature      string `json:"signature,omitempty"`
}

// PreauthorizationSaveRequest is the request for saving preauthorization result from Redsys
type PreauthorizationSaveRequest struct {
	OrderNumber       string `json:"order_number" validate:"required"`
	AuthorizationCode string `json:"authorization_code" validate:"omitempty"`
	Status            string `json:"status" validate:"required"`
	ErrorCode         string `json:"error_code" validate:"omitempty"`
	ErrorMessage      string `json:"error_message" validate:"omitempty"`
	MerchantData      string `json:"merchant_data" validate:"omitempty"`
}

func (r *PreauthorizationSaveRequest) Bind(_ *http.Request) error {
	return validate.Struct(r)
}

// CaptureOrderRequest is the request for capturing a preauthorized amount
type CaptureOrderRequest struct {
	OrderNumber string `json:"order_number" validate:"required"`
	Amount      int    `json:"amount" validate:"required,min=1"`
}

func (r *CaptureOrderRequest) Bind(_ *http.Request) error {
	return validate.Struct(r)
}

// CaptureOrderResponse is the response for capture operation
type CaptureOrderResponse struct {
	OrderNumber    string                 `json:"order_number"`
	CapturedAmount int                    `json:"captured_amount"`
	Status         PreauthorizationStatus `json:"status"`
	ErrorCode      string                 `json:"error_code,omitempty"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
}

// PreauthorizationUpdateRequest is the request for updating preauthorization status
type PreauthorizationUpdateRequest struct {
	OrderNumber string `json:"order_number" validate:"required"`
	Status      string `json:"status" validate:"required"`
}

func (r *PreauthorizationUpdateRequest) Bind(_ *http.Request) error {
	return validate.Struct(r)
}
