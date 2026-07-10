package core

import (
	"context"
	"errors"
	"testing"

	"evsys-back/entity"
	database_mock "evsys-back/impl/database-mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubMailer records the last SendTransaction call.
type stubMailer struct {
	sentTo   string
	sentData entity.TransactionMail
	calls    int
	err      error
}

func (s *stubMailer) SendNow(context.Context, *entity.MailSubscription) error { return nil }
func (s *stubMailer) SendTest(context.Context, string) error                  { return nil }
func (s *stubMailer) SendPaymentWarning(context.Context, string, entity.PaymentWarning) error {
	return nil
}

func (s *stubMailer) SendTransaction(_ context.Context, to string, t entity.TransactionMail) error {
	s.calls++
	s.sentTo = to
	s.sentData = t
	return s.err
}

const (
	ownerId     = "user-owner"
	strangerId  = "user-stranger"
	ownedTxId   = 4207
	unownedTxId = 4208
)

func newMailCore(t *testing.T) (*Core, *stubMailer) {
	t.Helper()
	db := database_mock.NewMockDB()
	db.SeedTransaction(&entity.Transaction{
		TransactionId: ownedTxId,
		ChargePointId: "PE00003",
		UserTag:       &entity.UserTag{UserId: ownerId, IdTag: "ABC123", Username: "owner"},
	})
	db.SeedTransaction(&entity.Transaction{
		TransactionId: unownedTxId,
		ChargePointId: "PE00003",
		UserTag:       &entity.UserTag{UserId: strangerId, IdTag: "XYZ789", Username: "stranger"},
	})

	c := New(newTestLogger(), db)
	m := &stubMailer{}
	c.SetMailService(m)
	return c, m
}

func TestSendTransactionMail(t *testing.T) {
	ctx := context.Background()
	owner := &entity.User{UserId: ownerId, Username: "owner", Role: "user"}
	admin := &entity.User{UserId: "admin-1", Username: "admin", Role: "admin"}

	t.Run("owner may mail own transaction", func(t *testing.T) {
		c, m := newMailCore(t)
		require.NoError(t, c.SendTransactionMail(ctx, owner, ownedTxId, "me@example.com"))
		assert.Equal(t, 1, m.calls)
		assert.Equal(t, "me@example.com", m.sentTo)
		assert.Equal(t, ownedTxId, m.sentData.Transaction.TransactionId)
	})

	t.Run("regular user may not mail another user's transaction", func(t *testing.T) {
		c, m := newMailCore(t)
		err := c.SendTransactionMail(ctx, owner, unownedTxId, "me@example.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
		assert.Zero(t, m.calls, "no mail may be sent on a denied request")
	})

	t.Run("power user may mail any transaction", func(t *testing.T) {
		c, m := newMailCore(t)
		require.NoError(t, c.SendTransactionMail(ctx, admin, unownedTxId, "ops@example.com"))
		assert.Equal(t, 1, m.calls)
	})

	t.Run("transaction without a user tag is owned by nobody", func(t *testing.T) {
		c, m := newMailCore(t)
		db := database_mock.NewMockDB()
		db.SeedTransaction(&entity.Transaction{TransactionId: 99, ChargePointId: "PE1"})
		c = New(newTestLogger(), db)
		c.SetMailService(m)

		err := c.SendTransactionMail(ctx, owner, 99, "me@example.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
		assert.Zero(t, m.calls)
	})

	t.Run("missing transaction reports not found", func(t *testing.T) {
		c, m := newMailCore(t)
		err := c.SendTransactionMail(ctx, admin, 123456, "ops@example.com")
		require.Error(t, err)
		assert.True(t, errors.Is(err, entity.ErrNotFound), "want ErrNotFound, got %v", err)
		assert.Zero(t, m.calls)
	})

	t.Run("rejects malformed recipient before any lookup", func(t *testing.T) {
		for _, addr := range []string{"", "not-an-email", "a@b", "a b@example.com"} {
			c, m := newMailCore(t)
			err := c.SendTransactionMail(ctx, admin, ownedTxId, addr)
			require.Error(t, err, "address %q must be rejected", addr)
			assert.Contains(t, err.Error(), "valid recipient email is required")
			assert.Zero(t, m.calls)
		}
	})

	t.Run("errors when mail service is not configured", func(t *testing.T) {
		c := New(newTestLogger(), database_mock.NewMockDB())
		err := c.SendTransactionMail(ctx, admin, ownedTxId, "ops@example.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mail service not configured")
	})

	t.Run("propagates sender failure", func(t *testing.T) {
		c, m := newMailCore(t)
		m.err = errors.New("brevo down")
		err := c.SendTransactionMail(ctx, owner, ownedTxId, "me@example.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "brevo down")
	})
}
