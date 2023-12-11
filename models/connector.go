package models

import "time"

type Connector struct {
	Id            int       `json:"connector_id" bson:"connector_id"`
	ChargePointId string    `json:"charge_point_id" bson:"charge_point_id"`
	Type          string    `json:"type" bson:"type"`
	Status        string    `json:"status" bson:"status"`
	StatusTime    time.Time `json:"status_time" bson:"status_time"`
	Info          string    `json:"info" bson:"info"`
	VendorId      string    `json:"vendor_id" bson:"vendor_id"`
	ErrorCode     string    `json:"error_code" bson:"error_code"`
	Power         int       `json:"power" bson:"power"`
	TransactionId int       `json:"current_transaction_id" bson:"current_transaction_id"`
}
