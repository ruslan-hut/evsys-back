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
	status   map[string]time.Time
}

func (sr *StatusReader) SetLogger(logger services.LogHandler) {
	sr.logger = logger
}

func (sr *StatusReader) SetDatabase(database services.Database) {
	sr.database = database
}

func NewStatusReader() *StatusReader {
	statusReader := StatusReader{
		status: make(map[string]time.Time),
	}
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

func (sr *StatusReader) SaveStatus(userId string) (time.Time, error) {
	timeStart := time.Now()
	sr.status[userId] = timeStart
	return timeStart, nil
}

func (sr *StatusReader) GetStatus(userId string) (time.Time, bool) {
	status, ok := sr.status[userId]
	if !ok {
		return time.Now(), false
	}
	return status, true
}

func (sr *StatusReader) ClearStatus(userId string) {
	delete(sr.status, userId)
}

func (sr *StatusReader) GetLastMeterValue(transactionId int) (*models.TransactionMeter, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	meter, err := sr.database.GetLastMeterValue(transactionId)
	if err != nil {
		return nil, err
	}
	return meter, nil
}
