package core

import (
	"context"
	"evsys-back/entity"
	database_mock "evsys-back/impl/database-mock"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNew(t *testing.T) {
	logger := newTestLogger()
	db := database_mock.NewMockDB()

	core := New(logger, db)

	assert.NotNil(t, core)
	assert.NotNil(t, core.repo)
}

// --- Payment Method Tests ---

func TestSavePaymentMethod(t *testing.T) {
	tests := []struct {
		name       string
		user       *entity.User
		pm         *entity.PaymentMethod
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "nil user",
			user:       nil,
			pm:         &entity.PaymentMethod{Identifier: "pm_123"},
			wantErr:    true,
			wantErrMsg: "user is nil",
		},
		{
			name:       "nil payment method",
			user:       &entity.User{UserId: "user123", Username: "testuser"},
			pm:         nil,
			wantErr:    true,
			wantErrMsg: "payment method is nil",
		},
		{
			name:    "valid save - user association enforced",
			user:    &entity.User{UserId: "user123", Username: "testuser"},
			pm:      &entity.PaymentMethod{Identifier: "pm_123", CardNumber: "****1234"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			core := New(newTestLogger(), db)

			err := core.SavePaymentMethod(context.Background(), tt.user, tt.pm)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				return
			}

			assert.NoError(t, err)
			// Verify user association was enforced
			assert.Equal(t, tt.user.UserId, tt.pm.UserId)
			assert.Equal(t, tt.user.Username, tt.pm.UserName)
		})
	}
}

func TestUpdatePaymentMethod(t *testing.T) {
	tests := []struct {
		name       string
		user       *entity.User
		pm         *entity.PaymentMethod
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "nil user",
			user:       nil,
			pm:         &entity.PaymentMethod{Identifier: "pm_123"},
			wantErr:    true,
			wantErrMsg: "user is nil",
		},
		{
			name:       "nil payment method",
			user:       &entity.User{UserId: "user123", Username: "testuser"},
			pm:         nil,
			wantErr:    true,
			wantErrMsg: "payment method is nil",
		},
		{
			name:    "valid update - user association enforced",
			user:    &entity.User{UserId: "user123", Username: "testuser"},
			pm:      &entity.PaymentMethod{Identifier: "pm_123", IsDefault: true},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			core := New(newTestLogger(), db)

			err := core.UpdatePaymentMethod(context.Background(), tt.user, tt.pm)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				return
			}

			assert.NoError(t, err)
			// Verify user association was enforced
			assert.Equal(t, tt.user.UserId, tt.pm.UserId)
			assert.Equal(t, tt.user.Username, tt.pm.UserName)
		})
	}
}

func TestDeletePaymentMethod(t *testing.T) {
	tests := []struct {
		name       string
		user       *entity.User
		pm         *entity.PaymentMethod
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "nil user",
			user:       nil,
			pm:         &entity.PaymentMethod{Identifier: "pm_123"},
			wantErr:    true,
			wantErrMsg: "user is nil",
		},
		{
			name:       "nil payment method",
			user:       &entity.User{UserId: "user123", Username: "testuser"},
			pm:         nil,
			wantErr:    true,
			wantErrMsg: "payment method is nil",
		},
		{
			name:    "valid delete - user isolation enforced",
			user:    &entity.User{UserId: "user123", Username: "testuser"},
			pm:      &entity.PaymentMethod{Identifier: "pm_123"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			core := New(newTestLogger(), db)

			err := core.DeletePaymentMethod(context.Background(), tt.user, tt.pm)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				return
			}

			assert.NoError(t, err)
			// Verify user isolation was enforced
			assert.Equal(t, tt.user.UserId, tt.pm.UserId)
		})
	}
}

func TestGetPaymentMethods(t *testing.T) {
	t.Run("returns user methods", func(t *testing.T) {
		db := database_mock.NewMockDB()
		core := New(newTestLogger(), db)

		// Seed payment methods for user
		pm := &entity.PaymentMethod{
			UserId:     "user123",
			Identifier: "pm_123",
			CardNumber: "****1234",
		}
		_ = db.SavePaymentMethod(context.Background(), pm)

		methods, err := core.GetPaymentMethods(context.Background(), "user123")

		assert.NoError(t, err)
		assert.NotNil(t, methods)
	})

	t.Run("returns nil for user without methods", func(t *testing.T) {
		db := database_mock.NewMockDB()
		core := New(newTestLogger(), db)

		methods, err := core.GetPaymentMethods(context.Background(), "nonexistent_user")

		assert.NoError(t, err)
		assert.Nil(t, methods)
	})
}

// --- Payment Order Tests ---

func TestSetOrder(t *testing.T) {
	tests := []struct {
		name       string
		user       *entity.User
		order      *entity.PaymentOrder
		setup      func(*database_mock.MockDB)
		wantErr    bool
		wantErrMsg string
		checkOrder func(*testing.T, *entity.PaymentOrder)
	}{
		{
			name:       "nil user",
			user:       nil,
			order:      &entity.PaymentOrder{Amount: 1000},
			wantErr:    true,
			wantErrMsg: "user is nil",
		},
		{
			name:       "nil order",
			user:       &entity.User{UserId: "user123", Username: "testuser"},
			order:      nil,
			wantErr:    true,
			wantErrMsg: "order is nil",
		},
		{
			name:    "new order - first order starts at 1200",
			user:    &entity.User{UserId: "user123", Username: "testuser"},
			order:   &entity.PaymentOrder{Amount: 1000},
			setup:   nil,
			wantErr: false,
			checkOrder: func(t *testing.T, order *entity.PaymentOrder) {
				assert.Equal(t, 1200, order.Order)
				assert.Equal(t, "user123", order.UserId)
				assert.Equal(t, "testuser", order.UserName)
				assert.False(t, order.TimeOpened.IsZero())
			},
		},
		{
			name:  "new order - increments from last order",
			user:  &entity.User{UserId: "user123", Username: "testuser"},
			order: &entity.PaymentOrder{Amount: 2000},
			setup: func(db *database_mock.MockDB) {
				db.SeedPaymentOrder(&entity.PaymentOrder{Order: 1500})
			},
			wantErr: false,
			checkOrder: func(t *testing.T, order *entity.PaymentOrder) {
				assert.Equal(t, 1501, order.Order)
			},
		},
		{
			name:  "update existing order",
			user:  &entity.User{UserId: "user123", Username: "testuser"},
			order: &entity.PaymentOrder{Order: 1300, Amount: 1500, IsCompleted: true},
			setup: func(db *database_mock.MockDB) {
				db.SeedPaymentOrder(&entity.PaymentOrder{Order: 1300, Amount: 1000})
			},
			wantErr: false,
			checkOrder: func(t *testing.T, order *entity.PaymentOrder) {
				assert.Equal(t, 1300, order.Order)
				assert.True(t, order.IsCompleted)
			},
		},
		{
			name:  "auto-close interrupted order for same transaction",
			user:  &entity.User{UserId: "user123", Username: "testuser"},
			order: &entity.PaymentOrder{TransactionId: 12345, Amount: 1000},
			setup: func(db *database_mock.MockDB) {
				// Existing unclosed order for the same transaction
				db.SeedPaymentOrder(&entity.PaymentOrder{
					Order:         1400,
					TransactionId: 12345,
					IsCompleted:   false,
				})
			},
			wantErr: false,
			checkOrder: func(t *testing.T, order *entity.PaymentOrder) {
				// Should be assigned a new order number
				assert.Equal(t, 1401, order.Order)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			core := New(newTestLogger(), db)

			if tt.setup != nil {
				tt.setup(db)
			}

			result, err := core.SetOrder(context.Background(), tt.user, tt.order)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)
			if tt.checkOrder != nil {
				tt.checkOrder(t, result)
			}
		})
	}
}

// --- Transaction Tests ---

func TestGetActiveTransactions(t *testing.T) {
	t.Run("returns empty array when no transactions", func(t *testing.T) {
		db := database_mock.NewMockDB()
		core := New(newTestLogger(), db)

		result, err := core.GetActiveTransactions(context.Background(), "user123")

		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Should return empty array, not nil
		states, ok := result.([]*entity.ChargeState)
		require.True(t, ok)
		assert.Empty(t, states)
	})

	t.Run("returns active transactions with CheckState called", func(t *testing.T) {
		db := database_mock.NewMockDB()
		core := New(newTestLogger(), db)

		// Setup: add user tag and transaction
		db.SeedUser(&entity.User{UserId: "user123", Username: "testuser"})
		_ = db.AddUserTag(context.Background(), &entity.UserTag{
			UserId: "user123",
			IdTag:  "TAG123",
		})
		db.SeedTransaction(&entity.Transaction{
			TransactionId: 1,
			IdTag:         "TAG123",
			IsFinished:    false,
		})
		db.SeedChargeState(&entity.ChargeState{
			TransactionId: 1,
			IsCharging:    true,
			Status:        "Charging",
			PowerRate:     7000,
		})

		result, err := core.GetActiveTransactions(context.Background(), "user123")

		assert.NoError(t, err)
		assert.NotNil(t, result)

		states, ok := result.([]*entity.ChargeState)
		require.True(t, ok)
		assert.Len(t, states, 1)
		assert.Equal(t, 7000, states[0].PowerRate) // Should retain PowerRate since IsCharging=true
	})
}

func TestGetTransactions(t *testing.T) {
	t.Run("returns empty array when no transactions", func(t *testing.T) {
		db := database_mock.NewMockDB()
		core := New(newTestLogger(), db)

		result, err := core.GetTransactions(context.Background(), "user123", "month")

		assert.NoError(t, err)
		assert.NotNil(t, result)

		txs, ok := result.([]*entity.Transaction)
		require.True(t, ok)
		assert.Empty(t, txs)
	})
}

func TestGetTransaction(t *testing.T) {
	t.Run("returns nil for non-existent transaction", func(t *testing.T) {
		db := database_mock.NewMockDB()
		core := New(newTestLogger(), db)

		result, err := core.GetTransaction(context.Background(), "user123", 10, 9999)

		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns transaction state with CheckState called", func(t *testing.T) {
		db := database_mock.NewMockDB()
		core := New(newTestLogger(), db)

		db.SeedChargeState(&entity.ChargeState{
			TransactionId: 123,
			IsCharging:    false,
			Status:        "Finishing",
			PowerRate:     5000, // Should be cleared by CheckState
		})

		result, err := core.GetTransaction(context.Background(), "user123", 10, 123)

		assert.NoError(t, err)
		assert.NotNil(t, result)

		state, ok := result.(*entity.ChargeState)
		require.True(t, ok)
		assert.Equal(t, 0, state.PowerRate) // PowerRate should be 0 since IsCharging=false
	})
}

// --- Access Control Tests ---

func TestGetLocations(t *testing.T) {
	t.Run("access denied for non-admin", func(t *testing.T) {
		db := database_mock.NewMockDB()
		core := New(newTestLogger(), db)

		_, err := core.GetLocations(context.Background(), 5)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})

	t.Run("admin access allowed", func(t *testing.T) {
		db := database_mock.NewMockDB()
		core := New(newTestLogger(), db)

		result, err := core.GetLocations(context.Background(), MaxAccessLevel)

		assert.NoError(t, err)
		assert.Nil(t, result) // Mock returns nil
	})
}

func TestSaveChargePoint(t *testing.T) {
	t.Run("nil charge point", func(t *testing.T) {
		db := database_mock.NewMockDB()
		core := New(newTestLogger(), db)

		err := core.SaveChargePoint(context.Background(), 10, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "charge point is nil")
	})

	t.Run("access denied - insufficient level", func(t *testing.T) {
		db := database_mock.NewMockDB()
		core := New(newTestLogger(), db)

		cp := &entity.ChargePoint{
			Id:          "cp1",
			AccessLevel: 10,
		}

		err := core.SaveChargePoint(context.Background(), 5, cp)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})

	t.Run("access level capped at max", func(t *testing.T) {
		db := database_mock.NewMockDB()
		core := New(newTestLogger(), db)

		cp := &entity.ChargePoint{
			Id:          "cp1",
			AccessLevel: 15, // Above max
		}

		err := core.SaveChargePoint(context.Background(), 10, cp)

		assert.NoError(t, err)
		assert.Equal(t, MaxAccessLevel, cp.AccessLevel)
	})
}
