package services

import (
	"evsys-back/models"
	"time"
)

type StatusReader interface {
	GetTransactionAfter(userId string, after time.Time) (*models.Transaction, error)
	GetTransaction(transactionId int) (*models.Transaction, error)
	GetLastMeterValue(transactionId int) (*models.TransactionMeter, error)

	SaveStatus(userId string, stage models.Stage, transactionId int) (time.Time, error)
	GetStatus(userId string) (*models.UserStatus, bool)
	ClearStatus(userId string)
}
