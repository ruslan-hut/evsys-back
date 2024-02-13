package models

type WsResponse struct {
	Status          ResponseStatus `json:"status"`
	Stage           ResponseStage  `json:"stage"`
	Info            string         `json:"info"`
	UserId          string         `json:"user_id"`
	Progress        int            `json:"progress"`
	Power           int            `json:"power"`
	Price           int            `json:"price"`
	Minute          int64          `json:"minute" bson:"minute"`
	Id              int            `json:"id"`
	Data            string         `json:"data"`
	ConnectorId     int            `json:"connector_id"`
	ConnectorStatus string         `json:"connector_status"`
}
