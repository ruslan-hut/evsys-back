package entity

type WsResponse struct {
	Status          ResponseStatus    `json:"status"`
	Stage           ResponseStage     `json:"stage"`
	Info            string            `json:"info,omitempty"`
	UserId          string            `json:"user_id,omitempty"`
	Progress        int               `json:"progress,omitempty"`
	Power           int               `json:"power,omitempty"`
	PowerRate       int               `json:"power_rate,omitempty"`
	SoC             int               `json:"soc,omitempty"`
	Price           int               `json:"price,omitempty"`
	Minute          int64             `json:"minute,omitempty"`
	Id              int               `json:"id,omitempty"`
	Data            string            `json:"data,omitempty"`
	ConnectorId     int               `json:"connector_id,omitempty"`
	ConnectorStatus string            `json:"connector_status,omitempty"`
	MeterValue      *TransactionMeter `json:"meter_value,omitempty"`
}
