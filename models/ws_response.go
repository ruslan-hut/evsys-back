package models

type WsResponse struct {
	Status ResponseStatus `json:"status"`
	Info   string         `json:"info"`
	UserId string         `json:"user_id"`
}
