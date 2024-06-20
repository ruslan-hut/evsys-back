package entity

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
	Token         string      `json:"token"`
	ChargePointId string      `json:"charge_point_id"`
	ConnectorId   int         `json:"connector_id"`
	TransactionId int         `json:"transaction_id"`
	Command       CommandName `json:"command"`
}
