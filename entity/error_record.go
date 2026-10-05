package entity

import "time"

const ErrorRecordType = "errorRecord"

// ErrorRecord is one connector error reported by a charge point, as evsys
// writes it to errors_log (evsys entity.ErrorData).
type ErrorRecord struct {
	Location        string    `json:"location" bson:"location"`
	ChargePointId   string    `json:"charge_point_id" bson:"charge_point_id"`
	ConnectorId     int       `json:"connector_id" bson:"connector_id"`
	ErrorCode       string    `json:"error_code" bson:"error_code"`
	Info            string    `json:"info,omitempty" bson:"info"`
	Status          string    `json:"status" bson:"status"`
	Timestamp       time.Time `json:"timestamp" bson:"timestamp"`
	VendorId        string    `json:"vendor_id,omitempty" bson:"vendor_id"`
	VendorErrorCode string    `json:"vendor_error_code,omitempty" bson:"vendor_error_code"`
}

// ErrorSummary counts the errors_log records of one error code on one
// connector over a period.
type ErrorSummary struct {
	ChargePointId   string    `json:"charge_point_id" bson:"charge_point_id"`
	ConnectorId     int       `json:"connector_id" bson:"connector_id"`
	ErrorCode       string    `json:"error_code" bson:"error_code"`
	VendorErrorCode string    `json:"vendor_error_code,omitempty" bson:"vendor_error_code"`
	Count           int       `json:"count" bson:"count"`
	First           time.Time `json:"first" bson:"first"`
	Last            time.Time `json:"last" bson:"last"`
	LastInfo        string    `json:"last_info,omitempty" bson:"last_info"`
}
