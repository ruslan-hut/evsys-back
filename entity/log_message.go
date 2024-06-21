package entity

import "time"

const LogMessageType = "logMessage"

type LogMessage struct {
	Time      string    `json:"time" bson:"time"`
	Level     string    `json:"level" bson:"level"`
	Category  string    `json:"category" bson:"category"`
	Text      string    `json:"text" bson:"text"`
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`
}
