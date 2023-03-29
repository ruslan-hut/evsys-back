package models

type WsMessage struct {
	Topic string `json:"topic"`
	Data  string `json:"data"`
}
