package entity

import "time"

type Connector struct {
	Id            int       `json:"connector_id" bson:"connector_id" validate:"min=0"`
	IdName        string    `json:"connector_id_name" bson:"connector_id_name" validate:"omitempty"`
	ChargePointId string    `json:"charge_point_id" bson:"charge_point_id" validate:"required"`
	Type          string    `json:"type" bson:"type" validate:"omitempty"`
	Status        string    `json:"status" bson:"status" validate:"omitempty,connector_status"`
	StatusTime    time.Time `json:"status_time" bson:"status_time"`
	State         string    `json:"state" bson:"state" validate:"omitempty"`
	Info          string    `json:"info" bson:"info" validate:"omitempty"`
	VendorId      string    `json:"vendor_id" bson:"vendor_id" validate:"omitempty"`
	ErrorCode     string    `json:"error_code" bson:"error_code" validate:"omitempty"`
	Power         int       `json:"power" bson:"power" validate:"min=0"`
	TransactionId int       `json:"current_transaction_id" bson:"current_transaction_id" validate:"min=0"`
}
