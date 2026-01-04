package entity

type WsResponse struct {
	Status          ResponseStatus    `json:"status" validate:"required"`
	Stage           ResponseStage     `json:"stage" validate:"omitempty"`
	Info            string            `json:"info,omitempty" validate:"omitempty"`
	UserId          string            `json:"user_id,omitempty" validate:"omitempty"`
	Progress        int               `json:"progress,omitempty" validate:"min=0,max=100"`
	Power           int               `json:"power,omitempty" validate:"min=0"`
	PowerRate       int               `json:"power_rate,omitempty" validate:"min=0"`
	SoC             int               `json:"soc,omitempty" validate:"min=0,max=100"`
	Price           int               `json:"price,omitempty" validate:"min=0"`
	Minute          int64             `json:"minute,omitempty" validate:"min=0"`
	Id              int               `json:"id,omitempty" validate:"min=0"`
	Data            string            `json:"data,omitempty" validate:"omitempty"`
	ConnectorId     int               `json:"connector_id,omitempty" validate:"min=0"`
	ConnectorStatus string            `json:"connector_status,omitempty" validate:"omitempty,connector_status"`
	MeterValue      *TransactionMeter `json:"meter_value,omitempty" validate:"omitempty"`
}
