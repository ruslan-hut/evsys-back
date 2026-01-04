package entity

import "evsys-back/internal/lib/validate"

type CommandName string

const (
	StartTransaction      CommandName = "StartTransaction"
	StopTransaction       CommandName = "StopTransaction"
	CheckStatus           CommandName = "CheckStatus"
	ListenTransaction     CommandName = "ListenTransaction"
	StopListenTransaction CommandName = "StopListenTransaction"
	ListenChargePoints    CommandName = "ListenChargePoints"
	ListenLog             CommandName = "ListenLog"
	PingConnection        CommandName = "PingConnection"
)

type UserRequest struct {
	Token         string      `json:"token" validate:"required"`
	ChargePointId string      `json:"charge_point_id" validate:"omitempty"`
	ConnectorId   int         `json:"connector_id" validate:"min=0"`
	TransactionId int         `json:"transaction_id" validate:"min=0"`
	Command       CommandName `json:"command" validate:"required,ws_command"`
}

// Validate validates the user request
func (u *UserRequest) Validate() error {
	return validate.Struct(u)
}
