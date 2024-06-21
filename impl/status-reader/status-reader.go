package statusreader

import (
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Repository interface {
	GetTransactionByTag(userId string, after time.Time) (*entity.Transaction, error)
	GetTransaction(transactionId int) (*entity.Transaction, error)
	GetMeterValues(transactionId int, from time.Time) ([]*entity.TransactionMeter, error)
	ReadLogAfter(timeStart time.Time) ([]*entity.FeatureMessage, error)
}

type StatusReader struct {
	logger   *slog.Logger
	database Repository
	status   map[string]*entity.UserStatus
	mux      *sync.Mutex
}

func New(log *slog.Logger, repo Repository) *StatusReader {
	statusReader := StatusReader{
		database: repo,
		logger:   log.With(sl.Module("impl.status-reader")),
		status:   make(map[string]*entity.UserStatus),
		mux:      &sync.Mutex{},
	}
	return &statusReader
}

func (sr *StatusReader) GetTransactionAfter(userId string, after time.Time) (*entity.Transaction, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	transaction, err := sr.database.GetTransactionByTag(userId, after)
	if err != nil {
		return &entity.Transaction{TransactionId: -1}, nil
	}
	return transaction, nil
}

func (sr *StatusReader) GetTransaction(transactionId int) (*entity.Transaction, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	transaction, err := sr.database.GetTransaction(transactionId)
	if err != nil {
		return nil, fmt.Errorf("reading database: %v", err)
	}
	return transaction, nil
}

func (sr *StatusReader) SaveStatus(userId string, stage entity.Stage, transactionId int) (time.Time, error) {
	sr.mux.Lock()
	defer sr.mux.Unlock()
	timeStart := time.Now()
	sr.status[userId] = &entity.UserStatus{
		UserId:        userId,
		Time:          timeStart,
		Stage:         stage,
		TransactionId: transactionId,
	}
	return timeStart, nil
}

func (sr *StatusReader) GetStatus(userId string) (*entity.UserStatus, bool) {
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

func (sr *StatusReader) GetLastMeterValues(transactionId int, from time.Time) ([]*entity.TransactionMeter, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	meter, err := sr.database.GetMeterValues(transactionId, from)
	if err != nil {
		return nil, err
	}
	return meter, nil
}

func (sr *StatusReader) ReadLogAfter(timeStart time.Time) ([]*entity.FeatureMessage, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	messages, err := sr.database.ReadLogAfter(timeStart)
	if err != nil {
		return nil, err
	}
	return messages, nil
}
