package statusreader

import (
	"context"
	"errors"
	"evsys-back/entity"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepository implements Repository interface for testing
type mockRepository struct {
	transactions     map[int]*entity.Transaction
	transactionByTag *entity.Transaction
	transactionErr   error
	meterValues      []entity.TransactionMeter
	meterErr         error
	logMessages      []*entity.FeatureMessage
	logErr           error
}

func (m *mockRepository) GetTransactionByTag(_ context.Context, _ string, _ time.Time) (*entity.Transaction, error) {
	if m.transactionErr != nil {
		return nil, m.transactionErr
	}
	return m.transactionByTag, nil
}

func (m *mockRepository) GetTransaction(_ context.Context, transactionId int) (*entity.Transaction, error) {
	if m.transactionErr != nil {
		return nil, m.transactionErr
	}
	if m.transactions != nil {
		tx, ok := m.transactions[transactionId]
		if ok {
			return tx, nil
		}
	}
	return nil, nil
}

func (m *mockRepository) GetMeterValues(_ context.Context, _ int, _ time.Time) ([]entity.TransactionMeter, error) {
	if m.meterErr != nil {
		return nil, m.meterErr
	}
	return m.meterValues, nil
}

func (m *mockRepository) ReadLogAfter(_ context.Context, _ time.Time) ([]*entity.FeatureMessage, error) {
	if m.logErr != nil {
		return nil, m.logErr
	}
	return m.logMessages, nil
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNew(t *testing.T) {
	logger := newTestLogger()
	repo := &mockRepository{}

	sr := New(logger, repo)

	assert.NotNil(t, sr)
	assert.NotNil(t, sr.status)
	assert.Equal(t, repo, sr.database)
}

func TestGetTransactionAfter(t *testing.T) {
	t.Run("nil database returns error", func(t *testing.T) {
		sr := &StatusReader{
			logger:   newTestLogger(),
			database: nil,
			status:   make(map[string]*entity.UserStatus),
		}

		_, err := sr.GetTransactionAfter(context.Background(), "user123", time.Now())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database is not set")
	})

	t.Run("transaction found", func(t *testing.T) {
		repo := &mockRepository{
			transactionByTag: &entity.Transaction{
				TransactionId: 123,
				IdTag:         "TAG123",
			},
		}
		sr := New(newTestLogger(), repo)

		tx, err := sr.GetTransactionAfter(context.Background(), "user123", time.Now())

		assert.NoError(t, err)
		assert.NotNil(t, tx)
		assert.Equal(t, 123, tx.TransactionId)
	})

	t.Run("transaction not found returns TransactionId -1", func(t *testing.T) {
		repo := &mockRepository{
			transactionByTag: nil,
		}
		sr := New(newTestLogger(), repo)

		tx, err := sr.GetTransactionAfter(context.Background(), "user123", time.Now())

		assert.NoError(t, err)
		assert.NotNil(t, tx)
		assert.Equal(t, -1, tx.TransactionId)
	})

	t.Run("database error returns TransactionId -1", func(t *testing.T) {
		repo := &mockRepository{
			transactionErr: errors.New("database error"),
		}
		sr := New(newTestLogger(), repo)

		tx, err := sr.GetTransactionAfter(context.Background(), "user123", time.Now())

		assert.NoError(t, err)
		assert.NotNil(t, tx)
		assert.Equal(t, -1, tx.TransactionId)
	})
}

func TestGetTransaction(t *testing.T) {
	t.Run("nil database returns error", func(t *testing.T) {
		sr := &StatusReader{
			logger:   newTestLogger(),
			database: nil,
			status:   make(map[string]*entity.UserStatus),
		}

		_, err := sr.GetTransaction(context.Background(), 123)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database is not set")
	})

	t.Run("transaction found", func(t *testing.T) {
		repo := &mockRepository{
			transactions: map[int]*entity.Transaction{
				123: {TransactionId: 123, IdTag: "TAG123"},
			},
		}
		sr := New(newTestLogger(), repo)

		tx, err := sr.GetTransaction(context.Background(), 123)

		assert.NoError(t, err)
		assert.NotNil(t, tx)
		assert.Equal(t, 123, tx.TransactionId)
	})

	t.Run("transaction not found returns error", func(t *testing.T) {
		repo := &mockRepository{
			transactions: map[int]*entity.Transaction{},
		}
		sr := New(newTestLogger(), repo)

		tx, err := sr.GetTransaction(context.Background(), 999)

		assert.Error(t, err)
		assert.Nil(t, tx)
		assert.Contains(t, err.Error(), "no transaction data")
	})
}

func TestStatusLifecycle(t *testing.T) {
	sr := New(newTestLogger(), &mockRepository{})

	t.Run("save and get status", func(t *testing.T) {
		timeStart, err := sr.SaveStatus("user123", entity.StageStart, 100)

		assert.NoError(t, err)
		assert.False(t, timeStart.IsZero())

		status, ok := sr.GetStatus("user123")

		assert.True(t, ok)
		assert.NotNil(t, status)
		assert.Equal(t, "user123", status.UserId)
		assert.Equal(t, entity.StageStart, status.Stage)
		assert.Equal(t, 100, status.TransactionId)
		assert.Equal(t, timeStart, status.Time)
	})

	t.Run("get non-existent status returns false", func(t *testing.T) {
		status, ok := sr.GetStatus("nonexistent")

		assert.False(t, ok)
		assert.Nil(t, status)
	})

	t.Run("clear status", func(t *testing.T) {
		// First, ensure status exists
		_, _ = sr.SaveStatus("user_to_clear", entity.StageListen, 200)
		status, ok := sr.GetStatus("user_to_clear")
		require.True(t, ok)
		require.NotNil(t, status)

		// Clear it
		sr.ClearStatus("user_to_clear")

		// Verify it's gone
		status, ok = sr.GetStatus("user_to_clear")
		assert.False(t, ok)
		assert.Nil(t, status)
	})

	t.Run("clear non-existent status is safe", func(t *testing.T) {
		// Should not panic
		sr.ClearStatus("never_existed")
	})
}

func TestStatusConcurrency(t *testing.T) {
	sr := New(newTestLogger(), &mockRepository{})

	// Test concurrent access to status map
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			userId := "user" + string(rune('0'+id))
			_, _ = sr.SaveStatus(userId, entity.StageStart, id*100)
			_, _ = sr.GetStatus(userId)
			sr.ClearStatus(userId)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestGetLastMeterValues(t *testing.T) {
	t.Run("nil database returns error", func(t *testing.T) {
		sr := &StatusReader{
			logger:   newTestLogger(),
			database: nil,
			status:   make(map[string]*entity.UserStatus),
		}

		_, err := sr.GetLastMeterValues(context.Background(), 123, time.Now())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database is not set")
	})

	t.Run("returns meter values", func(t *testing.T) {
		repo := &mockRepository{
			meterValues: []entity.TransactionMeter{
				{ConsumedEnergy: 1000, PowerRate: 7200},
				{ConsumedEnergy: 2000, PowerRate: 7200},
			},
		}
		sr := New(newTestLogger(), repo)

		values, err := sr.GetLastMeterValues(context.Background(), 123, time.Now())

		assert.NoError(t, err)
		assert.Len(t, values, 2)
	})

	t.Run("returns empty slice when no values", func(t *testing.T) {
		repo := &mockRepository{
			meterValues: []entity.TransactionMeter{},
		}
		sr := New(newTestLogger(), repo)

		values, err := sr.GetLastMeterValues(context.Background(), 123, time.Now())

		assert.NoError(t, err)
		assert.Empty(t, values)
	})

	t.Run("propagates database error", func(t *testing.T) {
		repo := &mockRepository{
			meterErr: errors.New("database error"),
		}
		sr := New(newTestLogger(), repo)

		_, err := sr.GetLastMeterValues(context.Background(), 123, time.Now())

		assert.Error(t, err)
	})
}

func TestReadLogAfter(t *testing.T) {
	t.Run("nil database returns error", func(t *testing.T) {
		sr := &StatusReader{
			logger:   newTestLogger(),
			database: nil,
			status:   make(map[string]*entity.UserStatus),
		}

		_, err := sr.ReadLogAfter(context.Background(), time.Now())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database is not set")
	})

	t.Run("returns log messages", func(t *testing.T) {
		repo := &mockRepository{
			logMessages: []*entity.FeatureMessage{
				{Feature: "StatusNotification"},
				{Feature: "MeterValues"},
			},
		}
		sr := New(newTestLogger(), repo)

		messages, err := sr.ReadLogAfter(context.Background(), time.Now())

		assert.NoError(t, err)
		assert.Len(t, messages, 2)
	})

	t.Run("propagates database error", func(t *testing.T) {
		repo := &mockRepository{
			logErr: errors.New("database error"),
		}
		sr := New(newTestLogger(), repo)

		_, err := sr.ReadLogAfter(context.Background(), time.Now())

		assert.Error(t, err)
	})
}
