package models

type CommandName string

const (
	StartTransaction CommandName = "StartTransaction"
	StopTransaction  CommandName = "StopTransaction"
)

type UserRequest struct {
	Token         string      `json:"token"`
	ChargePointId string      `json:"charge_point_id"`
	ConnectorId   int         `json:"connector_id"`
	TransactionId int         `json:"transaction_id"`
	Command       CommandName `json:"command"`
}
