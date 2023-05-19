package internal

import (
	"evsys-back/models"
	"evsys-back/services"
	"fmt"
	"time"
)

type StatusReader struct {
	logger   services.LogHandler
	database services.Database
}

func (sr *StatusReader) SetLogger(logger services.LogHandler) {
	sr.logger = logger
}

func (sr *StatusReader) SetDatabase(database services.Database) {
	sr.database = database
}

func NewStatusReader() *StatusReader {
	statusReader := StatusReader{}
	return &statusReader
}

func (sr *StatusReader) GetTransaction(userId string, after time.Time) (*models.Transaction, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	transaction, err := sr.database.GetTransactionByTag(userId, after)
	if err != nil {
		return &models.Transaction{TransactionId: -1}, nil
	}
	return transaction, nil
}
