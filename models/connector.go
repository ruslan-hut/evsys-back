package models

type Connector struct {
	Id            int    `json:"connector_id" bson:"connector_id"`
	ChargePointId string `json:"charge_point_id" bson:"charge_point_id"`
	Type          string `json:"type" bson:"type"`
	Status        string `json:"status" bson:"status"`
	Power         int    `json:"power" bson:"power"`
	TransactionId int    `json:"current_transaction_id" bson:"current_transaction_id"`
}
