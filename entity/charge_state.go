package entity

import (
	"time"
)

const chargePointStatusCharging = "Charging"

type ChargeState struct {
	TransactionId      int                `json:"transaction_id" bson:"transaction_id"`
	ConnectorId        int                `json:"connector_id" bson:"connector_id"`
	Connector          *Connector         `json:"connector" bson:"connector"`
	ChargePointId      string             `json:"charge_point_id" bson:"charge_point_id"`
	ChargePointTitle   string             `json:"charge_point_title" bson:"charge_point_title"`
	ChargePointAddress string             `json:"charge_point_address" bson:"charge_point_address"`
	TimeStarted        time.Time          `json:"time_started" bson:"time_started"`
	MeterStart         int                `json:"meter_start" bson:"meter_start"`
	Duration           int                `json:"duration" bson:"duration"`
	Consumed           int                `json:"consumed" bson:"consumed"`
	PowerRate          int                `json:"power_rate" bson:"power_rate"`
	Price              int                `json:"price" bson:"price"`
	Status             string             `json:"status" bson:"status"`
	IsCharging         bool               `json:"is_charging" bson:"is_charging"`
	CanStop            bool               `json:"can_stop" bson:"can_stop"`
	MeterValues        []TransactionMeter `json:"meter_values" bson:"meter_values"`
	IdTag              string             `json:"id_tag,omitempty" bson:"id_tag,omitempty"`
	PaymentPlan        *PaymentPlan       `json:"payment_plan,omitempty" bson:"payment_plan,omitempty"`
	PaymentMethod      *PaymentMethod     `json:"payment_method,omitempty" bson:"payment_method,omitempty"`
	PaymentOrders      []PaymentOrder     `json:"payment_orders,omitempty" bson:"payment_orders,omitempty"`
}

func (cs *ChargeState) CheckState() {
	if !cs.IsCharging || cs.Status != chargePointStatusCharging {
		cs.PowerRate = 0
	}
}
