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
	TransactionId   int                    `json:"transaction_id" bson:"transaction_id"`
	SessionId       string                 `json:"session_id,omitempty" bson:"session_id,omitempty"`
	IsFinished      bool                   `json:"is_finished" bson:"is_finished"`
	ConnectorId     int                    `json:"connector_id" bson:"connector_id"`
	ChargePointId   string                 `json:"charge_point_id" bson:"charge_point_id"`
	IdTag           string                 `json:"id_tag" bson:"id_tag"`
	ReservationId   *int                   `json:"reservation_id,omitempty" bson:"reservation_id,omitempty"`
	MeterStart      int                    `json:"meter_start" bson:"meter_start"`
	MeterStop       int                    `json:"meter_stop" bson:"meter_stop"`
	TimeStart       time.Time              `json:"time_start" bson:"time_start"`
	TimeStop        time.Time              `json:"time_stop" bson:"time_stop"`
	Reason          string                 `json:"reason,omitempty" bson:"reason,omitempty"`
	IdTagNote       string                 `json:"id_tag_note,omitempty" bson:"id_tag_note,omitempty"`
	Username        string                 `json:"username,omitempty" bson:"username,omitempty"`
	PaymentAmount   int                    `json:"payment_amount" bson:"payment_amount"`
	PaymentBilled   int                    `json:"payment_billed" bson:"payment_billed"`
	PaymentOrder    int                    `json:"payment_order" bson:"payment_order"`
	Plan            *PaymentPlan           `json:"payment_plan,omitempty" bson:"payment_plan,omitempty"`
	Tariff          *Tariff                `json:"tariff,omitempty" bson:"tariff,omitempty"`
	MeterValues     []TransactionMeter     `json:"meter_values" bson:"meter_values"`
	PaymentMethod   *PaymentMethod         `json:"payment_method,omitempty" bson:"payment_method,omitempty"`
	PaymentOrders   []PaymentOrder         `json:"payment_orders,omitempty" bson:"payment_orders,omitempty"`
	UserTag         *UserTag               `json:"user_tag,omitempty" bson:"user_tag,omitempty"`
	ProtocolVersion string                 `json:"protocol_version,omitempty" bson:"protocol_version,omitempty"`
	EvseId          *int                   `json:"evse_id,omitempty" bson:"evse_id,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
}
