package models

type CentralSystemResponse struct {
	Status        ResponseStatus `json:"status" bson:"status"`
	Info          string         `json:"info" bson:"info"`
	ChargePointId string         `json:"charge_point_id" bson:"charge_point_id"`
	ConnectorId   int            `json:"connector_id" bson:"connector_id"`
}

func NewCentralSystemResponse(chargePointId string, connectorId int) *CentralSystemResponse {
	return &CentralSystemResponse{
		Status:        Success,
		ChargePointId: chargePointId,
		ConnectorId:   connectorId,
	}
}
