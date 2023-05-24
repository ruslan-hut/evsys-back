package models

type WsResponse struct {
	Status   ResponseStatus `json:"status"`
	Stage    ResponseStage  `json:"stage"`
	Info     string         `json:"info"`
	UserId   string         `json:"user_id"`
	Progress int            `json:"progress"`
	Id       int            `json:"id"`
	Data     string         `json:"data"`
}
