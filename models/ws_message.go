package models

type WsMessage struct {
	Topic string `json:"topic"`
	Data  []byte `json:"data"`
}
