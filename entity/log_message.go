package entity

import "time"

const LogMessageType = "logMessage"

type LogMessage struct {
	Time      string    `json:"time" bson:"time" validate:"omitempty"`
	Level     string    `json:"level" bson:"level" validate:"omitempty"`
	Category  string    `json:"category" bson:"category" validate:"omitempty"`
	Text      string    `json:"text" bson:"text" validate:"omitempty"`
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`
}
