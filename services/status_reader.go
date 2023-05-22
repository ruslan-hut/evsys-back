package services

import (
	"evsys-back/models"
	"time"
)

type ClientConnectionStatus string

type StatusReader interface {
	GetTransaction(userId string, after time.Time) (*models.Transaction, error)
	SaveStatus(userId string) (time.Time, error)
	GetStatus(userId string) (time.Time, bool)
	ClearStatus(userId string)
	GetLastMeterValue(transactionId int) (*models.TransactionMeter, error)
}
