package services

import "time"

const LogMessageType = "logMessage"
const FeatureMessageType = "featureMessage"

type LogHandler interface {
	Debug(text string)
	Info(text string)
	Warn(text string)
	Error(event string, err error)
}

type LogMessage struct {
	Time     string `json:"time" bson:"time"`
	Level    string `json:"level" bson:"level"`
	Category string `json:"category" bson:"category"`
	Text     string `json:"text" bson:"text"`
}

func (lm *LogMessage) MessageType() string {
	return LogMessageType
}

func (lm *LogMessage) DataType() string {
	return LogMessageType
}

type FeatureMessage struct {
	Time          string    `json:"time" bson:"time"`
	Feature       string    `json:"feature" bson:"feature"`
	ChargePointId string    `json:"id" bson:"charge_point_id"`
	Text          string    `json:"text" bson:"text"`
	Timestamp     time.Time `json:"timestamp" bson:"timestamp"`
}

func (fm *FeatureMessage) MessageType() string {
	return FeatureMessageType
}

func (fm *FeatureMessage) DataType() string {
	return FeatureMessageType
}
