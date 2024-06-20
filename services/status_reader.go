package services

import (
	"evsys-back/entity"
	"time"
)

const LogMessageType = "logMessage"
const FeatureMessageType = "featureMessage"

type StatusReader interface {
	GetTransactionAfter(userId string, after time.Time) (*entity.Transaction, error)
	GetTransaction(transactionId int) (*entity.Transaction, error)
	GetLastMeterValues(transactionId int, from time.Time) ([]*entity.TransactionMeter, error)

	SaveStatus(userId string, stage entity.Stage, transactionId int) (time.Time, error)
	GetStatus(userId string) (*entity.UserStatus, bool)
	ClearStatus(userId string)

	ReadLogAfter(timeStart time.Time) ([]*FeatureMessage, error)
}

type LogMessage struct {
	Time      string    `json:"time" bson:"time"`
	Level     string    `json:"level" bson:"level"`
	Category  string    `json:"category" bson:"category"`
	Text      string    `json:"text" bson:"text"`
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`
}

type FeatureMessage struct {
	Time          string    `json:"time" bson:"time"`
	Feature       string    `json:"feature" bson:"feature"`
	ChargePointId string    `json:"id" bson:"charge_point_id"`
	Text          string    `json:"text" bson:"text"`
	Timestamp     time.Time `json:"timestamp" bson:"timestamp"`
	Importance    string    `json:"importance" bson:"importance"`
}
