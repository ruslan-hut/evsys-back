package entity

import "time"

type TransactionMeter struct {
	Id              int       `json:"transaction_id" bson:"transaction_id"`
	Value           int       `json:"value" bson:"value"`
	PowerRate       int       `json:"power_rate" bson:"power_rate"`
	BatteryLevel    int       `json:"battery_level" bson:"battery_level"`
	ConsumedEnergy  int       `json:"consumed_energy" bson:"consumed_energy"`
	Price           int       `json:"price" bson:"price"`
	Time            time.Time `json:"time" bson:"time"`
	Timestamp       int64     `json:"timestamp" bson:"timestamp"` // value not present in database, but used for clients
	Minute          int64     `json:"minute" bson:"minute"`
	Unit            string    `json:"unit" bson:"unit"`
	Measurand       string    `json:"measurand" bson:"measurand"`
	ConnectorId     int       `json:"connector_id" bson:"connector_id"`
	ConnectorStatus string    `json:"connector_status" bson:"connector_status"`
}
