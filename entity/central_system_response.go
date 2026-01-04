package entity

type CentralSystemResponse struct {
	Status        ResponseStatus `json:"status,omitempty" bson:"status" validate:"omitempty"`
	Info          string         `json:"info,omitempty" bson:"info" validate:"omitempty"`
	ChargePointId string         `json:"charge_point_id,omitempty" bson:"charge_point_id" validate:"omitempty"`
	ConnectorId   int            `json:"connector_id,omitempty" bson:"connector_id" validate:"min=0"`
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
