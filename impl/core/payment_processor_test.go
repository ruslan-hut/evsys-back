package core

import (
	"context"
	"errors"
	"evsys-back/entity"
	database_mock "evsys-back/impl/database-mock"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRedsys records the MIT payments it is asked for and declines nothing.
type stubRedsys struct {
	mu   sync.Mutex
	pays []PayRequest
}

func (s *stubRedsys) payRequests() []PayRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]PayRequest(nil), s.pays...)
}

func (s *stubRedsys) Pay(_ context.Context, req PayRequest) (*CaptureResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pays = append(s.pays, req)
	return &CaptureResponse{Success: true, ResponseCode: "0000", Order: req.OrderNumber}, nil
}
func (s *stubRedsys) Capture(context.Context, CaptureRequest) (*CaptureResponse, error) {
	return &CaptureResponse{Success: true}, nil
}
func (s *stubRedsys) Cancel(context.Context, CaptureRequest) (*CaptureResponse, error) {
	return &CaptureResponse{Success: true}, nil
}
func (s *stubRedsys) Preauthorize(context.Context, PreauthorizeRequest) (*CaptureResponse, error) {
	return &CaptureResponse{Success: true}, nil
}
func (s *stubRedsys) Refund(context.Context, RefundRequest) (*CaptureResponse, error) {
	return &CaptureResponse{Success: true}, nil
}
func (s *stubRedsys) BuildEntryForm(EntryFormRequest) (*EntryFormResponse, error) {
	return &EntryFormResponse{}, nil
}
func (s *stubRedsys) VerifyNotification(string, string, string) error { return nil }

// unbilled builds a core with one finished, unpaid session of user_389b1 and
// the given cards on file.
func unbilled(t *testing.T, cards ...*entity.PaymentMethod) (*Core, *database_mock.MockDB, *stubRedsys) {
	t.Helper()
	db := database_mock.NewMockDB()
	redsys := &stubRedsys{}
	c := New(newTestLogger(), db)
	c.SetRedsys(redsys)
	for _, card := range cards {
		card.UserId = "uid-389b1"
		require.NoError(t, db.SavePaymentMethod(context.Background(), card))
	}
	db.SeedTransaction(&entity.Transaction{
		TransactionId: 5092, ChargePointId: "PE00001", ConnectorId: 1, IdTag: "TAG", IsFinished: true,
		MeterStart: 0, MeterStop: 67204, PaymentAmount: 3024,
		UserTag: &entity.UserTag{IdTag: "TAG", UserId: "uid-389b1", Username: "user_389b1"},
	})
	return c, db, redsys
}

func waitForPayments(t *testing.T, redsys *stubRedsys, n int) []PayRequest {
	t.Helper()
	// the payment request goes out asynchronously
	require.Eventually(t, func() bool { return len(redsys.payRequests()) == n }, 2*time.Second, 10*time.Millisecond)
	return redsys.payRequests()
}

// Production on 2026-10-03: the user's only card had just been declined
// (fail_count 1), so no card without failures was left. The nil method this
// lookup returned used to take the whole process down; now the declined card
// is charged again, since it is all there is.
func TestUnbilledFallsBackToFailedCard(t *testing.T) {
	c, _, redsys := unbilled(t, &entity.PaymentMethod{Identifier: "declined-card", CofTid: "cof", FailCount: 2, ExpiryDate: "3012"})

	require.NotPanics(t, func() { c.processUnbilledTransactions(context.Background()) })

	req := waitForPayments(t, redsys, 1)[0]
	assert.Equal(t, "declined-card", req.CardToken)
	assert.Equal(t, 3024, req.Amount)
}

func TestKnownCardChoice(t *testing.T) {
	c, _, _ := unbilled(t,
		&entity.PaymentMethod{Identifier: "worst", CofTid: "c", FailCount: 3, ExpiryDate: "3012"},
		&entity.PaymentMethod{Identifier: "expired", CofTid: "c", FailCount: 1, ExpiryDate: "2001"},
		&entity.PaymentMethod{Identifier: "no-cof", FailCount: 1, ExpiryDate: "3012"},
		&entity.PaymentMethod{Identifier: "fewer", CofTid: "c", FailCount: 1, ExpiryDate: "3012"},
		&entity.PaymentMethod{Identifier: "fewer-default", CofTid: "c", FailCount: 1, ExpiryDate: "3012", IsDefault: true},
	)
	pm := c.pickKnownPaymentMethod(context.Background(), "uid-389b1")
	require.NotNil(t, pm)
	assert.Equal(t, "fewer-default", pm.Identifier, "fewest failures, default wins the tie, unchargeable cards skipped")
}

func TestUnbilledWithoutChargeableCard(t *testing.T) {
	// an expired card and one without a network transaction id cannot be charged
	c, db, redsys := unbilled(t,
		&entity.PaymentMethod{Identifier: "expired", CofTid: "c", FailCount: 1, ExpiryDate: "2001"},
		&entity.PaymentMethod{Identifier: "no-cof", FailCount: 1, ExpiryDate: "3012"},
	)
	ctx := context.Background()
	c.processUnbilledTransactions(ctx)

	tx, err := db.GetTransaction(ctx, 5092)
	require.NoError(t, err)
	assert.Equal(t, 3024, tx.PaymentBilled, "recorded like a declined payment")
	assert.Equal(t, "no usable payment method", tx.PaymentError)

	retry, err := db.GetPaymentRetry(ctx, 5092)
	require.NoError(t, err)
	require.NotNil(t, retry, "the retry queue takes over")
	assert.Equal(t, 1, retry.Attempt)
	assert.Empty(t, redsys.payRequests(), "nothing was sent to Redsys")

	// the next pass leaves it to the retry queue
	c.processUnbilledTransactions(ctx)
	retry, _ = db.GetPaymentRetry(ctx, 5092)
	assert.Equal(t, 1, retry.Attempt)
}

func TestRetryChargesCardAddedLater(t *testing.T) {
	c, db, redsys := unbilled(t)
	ctx := context.Background()
	c.processUnbilledTransactions(ctx)
	require.Empty(t, redsys.payRequests())

	require.NoError(t, db.SavePaymentMethod(ctx, &entity.PaymentMethod{
		Identifier: "new-card", UserId: "uid-389b1", CofTid: "cof-2", ExpiryDate: "3012", IsDefault: true,
	}))
	require.NoError(t, c.retryOne(ctx, 5092, 1))

	req := waitForPayments(t, redsys, 1)[0]
	assert.Equal(t, "new-card", req.CardToken)
	assert.Equal(t, 3024, req.Amount)
}

func TestGuardPaymentContainsPanic(t *testing.T) {
	c := New(newTestLogger(), database_mock.NewMockDB())
	var err error
	require.NotPanics(t, func() {
		err = c.guardPayment(context.Background(), 7, func() error {
			var pm *entity.PaymentMethod
			_ = pm.CofTid
			return nil
		})
	})
	assert.ErrorContains(t, err, "transaction 7: panic")
}

// failingCardLookup is the mock DB with the payment-method lookup broken, as
// on a transient MongoDB error.
type failingCardLookup struct {
	*database_mock.MockDB
}

func (failingCardLookup) GetDefaultPaymentMethod(context.Context, string) (*entity.PaymentMethod, error) {
	return nil, errors.New("server selection timeout")
}

func TestCardLookupErrorIsRetriedNotWrittenOff(t *testing.T) {
	db := database_mock.NewMockDB()
	redsys := &stubRedsys{}
	c := New(newTestLogger(), failingCardLookup{db})
	c.SetRedsys(redsys)
	db.SeedTransaction(&entity.Transaction{
		TransactionId: 5093, ChargePointId: "PE00003", IdTag: "TAG", IsFinished: true,
		MeterStop: 41607, PaymentAmount: 1872,
		UserTag: &entity.UserTag{IdTag: "TAG", UserId: "uid-0d068", Username: "user_0d068"},
	})
	ctx := context.Background()

	err := c.PayTransaction(ctx, 5093)
	assert.ErrorContains(t, err, "payment method lookup failed")

	tx, _ := db.GetTransaction(ctx, 5093)
	assert.Equal(t, "payment method lookup failed", tx.PaymentError, "the failure is visible, not a silent write-off")
	retry, _ := db.GetPaymentRetry(ctx, 5093)
	require.NotNil(t, retry)
	assert.Equal(t, 1, retry.Attempt)
	assert.Empty(t, redsys.payRequests())
}
