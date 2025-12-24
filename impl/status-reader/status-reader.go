package statusreader

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Repository interface {
	GetTransactionByTag(ctx context.Context, userId string, after time.Time) (*entity.Transaction, error)
	GetTransaction(ctx context.Context, transactionId int) (*entity.Transaction, error)
	GetMeterValues(ctx context.Context, transactionId int, from time.Time) ([]*entity.TransactionMeter, error)
	ReadLogAfter(ctx context.Context, timeStart time.Time) ([]*entity.FeatureMessage, error)
}

type StatusReader struct {
	logger   *slog.Logger
	database Repository
	status   map[string]*entity.UserStatus
	mux      sync.Mutex
}

func New(log *slog.Logger, repo Repository) *StatusReader {
	statusReader := StatusReader{
		database: repo,
		logger:   log.With(sl.Module("impl.status-reader")),
		status:   make(map[string]*entity.UserStatus),
		mux:      sync.Mutex{},
	}
	return &statusReader
}

func (sr *StatusReader) GetTransactionAfter(ctx context.Context, userId string, after time.Time) (*entity.Transaction, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	transaction, err := sr.database.GetTransactionByTag(ctx, userId, after)
	if err != nil || transaction == nil {
		return &entity.Transaction{TransactionId: -1}, nil
	}
	return transaction, nil
}

func (sr *StatusReader) GetTransaction(ctx context.Context, transactionId int) (*entity.Transaction, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	transaction, err := sr.database.GetTransaction(ctx, transactionId)
	if err != nil || transaction == nil {
		return nil, fmt.Errorf("no transaction data: %d", transactionId)
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
	sr.mux.Lock()
	defer sr.mux.Unlock()
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

func (sr *StatusReader) GetLastMeterValues(ctx context.Context, transactionId int, from time.Time) ([]*entity.TransactionMeter, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	meter, err := sr.database.GetMeterValues(ctx, transactionId, from)
	if err != nil {
		return nil, err
	}
	return meter, nil
}

func (sr *StatusReader) ReadLogAfter(ctx context.Context, timeStart time.Time) ([]*entity.FeatureMessage, error) {
	if sr.database == nil {
		return nil, fmt.Errorf("database is not set for status reader")
	}
	messages, err := sr.database.ReadLogAfter(ctx, timeStart)
	if err != nil {
		return nil, err
	}
	return messages, nil
}
