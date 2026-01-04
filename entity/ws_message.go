package entity

type WsMessage struct {
	Topic  string `json:"topic" validate:"required"`
	Data   string `json:"data" validate:"omitempty"`
	UserId string `json:"user_id" validate:"omitempty"`
}
