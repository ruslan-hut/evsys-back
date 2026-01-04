package entity

import "time"

type TransactionMeter struct {
	Id              int       `json:"transaction_id" bson:"transaction_id" validate:"required,min=1"`
	Value           int       `json:"value" bson:"value" validate:"min=0"`
	PowerRate       int       `json:"power_rate" bson:"power_rate" validate:"min=0"`
	BatteryLevel    int       `json:"battery_level" bson:"battery_level" validate:"min=0,max=100"`
	ConsumedEnergy  int       `json:"consumed_energy" bson:"consumed_energy" validate:"min=0"`
	Price           int       `json:"price" bson:"price" validate:"min=0"`
	Time            time.Time `json:"time" bson:"time"`
	Timestamp       int64     `json:"timestamp" bson:"timestamp"` // value not present in database, but used for clients
	Minute          int64     `json:"minute" bson:"minute" validate:"min=0"`
	Unit            string    `json:"unit" bson:"unit" validate:"omitempty"`
	Measurand       string    `json:"measurand" bson:"measurand" validate:"omitempty"`
	ConnectorId     int       `json:"connector_id" bson:"connector_id" validate:"min=0"`
	ConnectorStatus string    `json:"connector_status" bson:"connector_status" validate:"omitempty,connector_status"`
}
