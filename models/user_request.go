package models

type CommandName string

const (
	StartTransaction CommandName = "StartTransaction"
	StopTransaction  CommandName = "StopTransaction"
)

type UserRequest struct {
	Username      string      `json:"username"`
	UserId        string      `json:"user_id"`
	ChargePointId string      `json:"charge_point_id"`
	ConnectorId   int         `json:"connector_id"`
	TransactionId int         `json:"transaction_id"`
	Command       CommandName `json:"command"`
}
