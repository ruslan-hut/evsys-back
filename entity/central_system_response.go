package entity

type CentralSystemResponse struct {
	Status        ResponseStatus `json:"status,omitempty" bson:"status"`
	Info          string         `json:"info,omitempty" bson:"info"`
	ChargePointId string         `json:"charge_point_id,omitempty" bson:"charge_point_id"`
	ConnectorId   int            `json:"connector_id,omitempty" bson:"connector_id"`
}

func NewCentralSystemResponse(chargePointId string, connectorId int) *CentralSystemResponse {
	return &CentralSystemResponse{
		Status:        Success,
		ChargePointId: chargePointId,
		ConnectorId:   connectorId,
	}
}

func (r *CentralSystemResponse) SetError(info string) {
	r.Status = Error
	r.Info = info
}

func (r *CentralSystemResponse) IsError() bool {
	return r.Status == Error
}
