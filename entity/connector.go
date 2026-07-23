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
	TransactionId int       `json:"current_transaction_id" bson:"current_transaction_id"`
	// CurrentPowerLimit is the active charging current limit in amperes.
	CurrentPowerLimit int `json:"current_power_limit" bson:"current_power_limit" validate:"min=0"`
	// LastProfile is the charge point's answer to the last limit installed here.
	// CurrentPowerLimit records what the central system asked for and is written
	// before the answer arrives, so on its own it cannot distinguish a limit in
	// force from one the charge point refused. Nil on connectors never sent a
	// profile, and on every connector written before evsys recorded the answer.
	LastProfile *ProfileVerdict `json:"last_profile,omitempty" bson:"last_profile,omitempty" validate:"omitempty"`
}

// Statuses reported for an installed charging profile. The first three are
// OCPP's own; the rest describe answers that never arrived or made no sense,
// which are just as much a failure to enforce a limit.
const (
	ProfileStatusAccepted     = "Accepted"
	ProfileStatusRejected     = "Rejected"
	ProfileStatusNotSupported = "NotSupported"
	ProfileStatusNoResponse   = "NoResponse"
	ProfileStatusUnreadable   = "Unreadable"
)

// ProfileVerdict is what a charge point said about a charging profile.
type ProfileVerdict struct {
	Status     string    `json:"status" bson:"status" validate:"omitempty"`
	Limit      int       `json:"limit" bson:"limit" validate:"min=0"`
	StackLevel int       `json:"stack_level" bson:"stack_level" validate:"min=0"`
	Time       time.Time `json:"time" bson:"time"`
}

// Accepted reports whether the charge point took the profile.
func (v *ProfileVerdict) Accepted() bool {
	return v != nil && v.Status == ProfileStatusAccepted
}
