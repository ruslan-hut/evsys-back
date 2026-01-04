package entity

import "time"

// TransactionFilter represents query parameters for filtering transactions
type TransactionFilter struct {
	From          *time.Time // Start date filter (inclusive)
	To            *time.Time // End date filter (inclusive)
	Username      string     // Filter by username (via UserTag.username)
	IdTag         string     // Filter by RFID tag ID
	ChargePointId string     // Filter by charge point identifier
}

// HasFilters returns true if any filter is set
func (f *TransactionFilter) HasFilters() bool {
	return f.From != nil || f.To != nil || f.Username != "" || f.IdTag != "" || f.ChargePointId != ""
}

type Transaction struct {
	TransactionId   int                    `json:"transaction_id" bson:"transaction_id" validate:"required,min=1"`
	SessionId       string                 `json:"session_id,omitempty" bson:"session_id,omitempty" validate:"omitempty"`
	IsFinished      bool                   `json:"is_finished" bson:"is_finished"`
	ConnectorId     int                    `json:"connector_id" bson:"connector_id" validate:"min=0"`
	ChargePointId   string                 `json:"charge_point_id" bson:"charge_point_id" validate:"required"`
	IdTag           string                 `json:"id_tag" bson:"id_tag" validate:"required"`
	ReservationId   *int                   `json:"reservation_id,omitempty" bson:"reservation_id,omitempty" validate:"omitempty,min=0"`
	MeterStart      int                    `json:"meter_start" bson:"meter_start" validate:"min=0"`
	MeterStop       int                    `json:"meter_stop" bson:"meter_stop" validate:"min=0"`
	TimeStart       time.Time              `json:"time_start" bson:"time_start"`
	TimeStop        time.Time              `json:"time_stop" bson:"time_stop"`
	Reason          string                 `json:"reason,omitempty" bson:"reason,omitempty" validate:"omitempty,stop_reason"`
	IdTagNote       string                 `json:"id_tag_note,omitempty" bson:"id_tag_note,omitempty" validate:"omitempty"`
	Username        string                 `json:"username,omitempty" bson:"username,omitempty" validate:"omitempty"`
	PaymentAmount   int                    `json:"payment_amount" bson:"payment_amount" validate:"min=0"`
	PaymentBilled   int                    `json:"payment_billed" bson:"payment_billed" validate:"min=0"`
	PaymentOrder    int                    `json:"payment_order" bson:"payment_order" validate:"min=0"`
	Plan            *PaymentPlan           `json:"payment_plan,omitempty" bson:"payment_plan,omitempty" validate:"omitempty"`
	Tariff          *Tariff                `json:"tariff,omitempty" bson:"tariff,omitempty" validate:"omitempty"`
	MeterValues     []TransactionMeter     `json:"meter_values" bson:"meter_values" validate:"omitempty,dive"`
	PaymentMethod   *PaymentMethod         `json:"payment_method,omitempty" bson:"payment_method,omitempty" validate:"omitempty"`
	PaymentOrders   []PaymentOrder         `json:"payment_orders,omitempty" bson:"payment_orders,omitempty" validate:"omitempty,dive"`
	UserTag         *UserTag               `json:"user_tag,omitempty" bson:"user_tag,omitempty" validate:"omitempty"`
	ProtocolVersion string                 `json:"protocol_version,omitempty" bson:"protocol_version,omitempty" validate:"omitempty"`
	EvseId          *int                   `json:"evse_id,omitempty" bson:"evse_id,omitempty" validate:"omitempty,min=0"`
	Metadata        map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
}
