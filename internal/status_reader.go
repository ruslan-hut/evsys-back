package internal

import (
	"evsys-back/models"
	"evsys-back/services"
	"fmt"
	"sync"
	"time"
)

type StatusReader struct {
	logger   services.LogHandler
	database services.Database
	status   map[string]*models.UserStatus
	mux      *sync.Mutex
}

func (sr *StatusReader) SetLogger(logger services.LogHandler) {
	sr.logger = logger
}

func (sr *StatusReader) SetDatabase(database services.Database) {
	sr.database = database
}

func NewStatusReader() *StatusReader {
	statusReader := StatusReader{
		status: make(map[string]*models.UserStatus),
		mux:    &sync.Mutex{},
	}
	return &statusReader
}

func (sr *StatusReader) GetTransactionAfter(userId string, after time.Time) (*models.Transaction, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	transaction, err := sr.database.GetTransactionByTag(userId, after)
	if err != nil {
		return &models.Transaction{TransactionId: -1}, nil
	}
	return transaction, nil
}

func (sr *StatusReader) GetTransaction(transactionId int) (*models.Transaction, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	transaction, err := sr.database.GetTransaction(transactionId)
	if err != nil {
		return nil, fmt.Errorf("reading database: %v", err)
	}
	return transaction, nil
}

func (sr *StatusReader) SaveStatus(userId string, stage models.Stage, transactionId int) (time.Time, error) {
	sr.mux.Lock()
	defer sr.mux.Unlock()
	timeStart := time.Now()
	sr.status[userId] = &models.UserStatus{
		UserId:        userId,
		Time:          timeStart,
		Stage:         stage,
		TransactionId: transactionId,
	}
	return timeStart, nil
}

func (sr *StatusReader) GetStatus(userId string) (*models.UserStatus, bool) {
	status, ok := sr.status[userId]
	if !ok {
		return nil, false
	}
	return status, true
}

func (sr *StatusReader) ClearStatus(userId string) {
	sr.mux.Lock()
	defer sr.mux.Unlock()
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

func (sr *StatusReader) ReadLogAfter(timeStart time.Time) ([]*services.FeatureMessage, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	messages, err := sr.database.ReadLogAfter(timeStart)
	if err != nil {
		return nil, err
	}
	return messages, nil
}
