package entity

import (
	"time"
)

const chargePointStatusCharging = "Charging"

type ChargeState struct {
	TransactionId      int                `json:"transaction_id" bson:"transaction_id" validate:"required,min=1"`
	ConnectorId        int                `json:"connector_id" bson:"connector_id" validate:"min=0"`
	Connector          *Connector         `json:"connector" bson:"connector" validate:"omitempty"`
	ChargePointId      string             `json:"charge_point_id" bson:"charge_point_id" validate:"required"`
	ChargePointTitle   string             `json:"charge_point_title" bson:"charge_point_title" validate:"omitempty"`
	ChargePointAddress string             `json:"charge_point_address" bson:"charge_point_address" validate:"omitempty"`
	TimeStarted        time.Time          `json:"time_started" bson:"time_started"`
	MeterStart         int                `json:"meter_start" bson:"meter_start" validate:"min=0"`
	Duration           int                `json:"duration" bson:"duration" validate:"min=0"`
	Consumed           int                `json:"consumed" bson:"consumed" validate:"min=0"`
	PowerRate          int                `json:"power_rate" bson:"power_rate" validate:"min=0"`
	Price              int                `json:"price" bson:"price" validate:"min=0"`
	PaymentBilled      int                `json:"payment_billed" bson:"payment_billed" validate:"min=0"`
	Status             string             `json:"status" bson:"status" validate:"omitempty,connector_status"`
	IsCharging         bool               `json:"is_charging" bson:"is_charging"`
	CanStop            bool               `json:"can_stop" bson:"can_stop"`
	MeterValues        []TransactionMeter `json:"meter_values" bson:"meter_values" validate:"omitempty,dive"`
	IdTag              string             `json:"id_tag,omitempty" bson:"id_tag,omitempty" validate:"omitempty"`
	PaymentPlan        *PaymentPlan       `json:"payment_plan,omitempty" bson:"payment_plan,omitempty" validate:"omitempty"`
	PaymentMethod      *PaymentMethod     `json:"payment_method,omitempty" bson:"payment_method,omitempty" validate:"omitempty"`
	PaymentOrders      []PaymentOrder     `json:"payment_orders,omitempty" bson:"payment_orders,omitempty" validate:"omitempty,dive"`
}

func (cs *ChargeState) CheckState() {
	if !cs.IsCharging || cs.Status != chargePointStatusCharging {
		cs.PowerRate = 0
	}
}
