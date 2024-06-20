package services

import (
	"evsys-back/entity"
	"time"
)

type StatusReader interface {
	GetTransactionAfter(userId string, after time.Time) (*entity.Transaction, error)
	GetTransaction(transactionId int) (*entity.Transaction, error)
	GetLastMeterValues(transactionId int, from time.Time) ([]*entity.TransactionMeter, error)

	SaveStatus(userId string, stage entity.Stage, transactionId int) (time.Time, error)
	GetStatus(userId string) (*entity.UserStatus, bool)
	ClearStatus(userId string)

	ReadLogAfter(timeStart time.Time) ([]*FeatureMessage, error)
}
