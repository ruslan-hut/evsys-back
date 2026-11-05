package core

import (
	"context"
	"errors"
	"evsys-back/entity"
	database_mock "evsys-back/impl/database-mock"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRedsys records the MIT payments it is asked for. It approves them,
// unless decline holds a Redsys error code, or declines[i] does for the i-th
// request.
type stubRedsys struct {
	mu       sync.Mutex
	pays     []PayRequest
	decline  string
	declines []string
}

func (s *stubRedsys) payRequests() []PayRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]PayRequest(nil), s.pays...)
}

func (s *stubRedsys) Pay(_ context.Context, req PayRequest) (*CaptureResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.pays)
	s.pays = append(s.pays, req)
	code := s.decline
	if n < len(s.declines) && s.declines[n] != "" {
		code = s.declines[n]
	}
	if code != "" {
		return &CaptureResponse{Success: false, ErrorCode: code}, nil
	}
	return &CaptureResponse{Success: true, ResponseCode: "0000", Order: req.OrderNumber, Amount: strconv.Itoa(req.Amount)}, nil
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

// --- starting a session settles what the user owes ---

// startRequest is the app asking to start a session with user_389b1's tag.
var startRequest = &entity.UserRequest{Command: entity.StartTransaction, Token: "TAG", ChargePointId: "PE00003", ConnectorId: 1}

func withTag(t *testing.T, db *database_mock.MockDB) {
	t.Helper()
	require.NoError(t, db.AddUserTag(context.Background(), &entity.UserTag{IdTag: "TAG", UserId: "uid-389b1", Username: "user_389b1"}))
}

func TestStartChargesPendingSessionFirst(t *testing.T) {
	c, db, redsys := unbilled(t, &entity.PaymentMethod{Identifier: "good-card", CofTid: "cof", ExpiryDate: "3012", IsDefault: true})
	withTag(t, db)
	ctx := context.Background()

	require.NoError(t, c.validateStartTransactionPaymentMethod(ctx, startRequest))

	require.Len(t, redsys.payRequests(), 1, "the previous session was charged before the start")
	tx, _ := db.GetTransaction(ctx, 5092)
	assert.Equal(t, 3024, tx.PaymentBilled)
	assert.Empty(t, tx.PaymentError)

	// nothing owed any more: the next start sends nothing
	require.NoError(t, c.validateStartTransactionPaymentMethod(ctx, startRequest))
	assert.Len(t, redsys.payRequests(), 1)
}

// 5091 and 5092: the card is declined when the pending session is charged, so
// the second session does not start.
func TestStartRefusedWhenPendingPaymentDeclined(t *testing.T) {
	c, db, redsys := unbilled(t, &entity.PaymentMethod{Identifier: "fresh-card", CofTid: "cof", ExpiryDate: "3012", IsDefault: true})
	withTag(t, db)
	redsys.decline = "SIS0334"

	err := c.validateStartTransactionPaymentMethod(context.Background(), startRequest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "5092")
	assert.Contains(t, err.Error(), "SIS0334")
}

func TestStartRefusedWithDebtAndNoNewCard(t *testing.T) {
	c, db, redsys := unbilled(t, &entity.PaymentMethod{Identifier: "declined-card", CofTid: "cof", FailCount: 1, ExpiryDate: "3012"})
	withTag(t, db)
	ctx := context.Background()
	redsys.decline = "SIS0334"
	c.processUnbilledTransactions(ctx) // declined: now a failed payment in the retry queue
	waitForPayments(t, redsys, 1)
	require.Eventually(t, func() bool {
		r, _ := db.GetPaymentRetry(ctx, 5092)
		return r != nil
	}, 2*time.Second, 10*time.Millisecond)

	err := c.validateStartTransactionPaymentMethod(ctx, startRequest)
	assert.ErrorContains(t, err, "add a new payment method")
	assert.Len(t, redsys.payRequests(), 1, "the declined card is not tried again on start")
}

func TestStartChargesDebtOnNewCard(t *testing.T) {
	c, db, redsys := unbilled(t, &entity.PaymentMethod{Identifier: "declined-card", CofTid: "cof", FailCount: 1, ExpiryDate: "3012"})
	withTag(t, db)
	ctx := context.Background()
	redsys.decline = "SIS0334"
	c.processUnbilledTransactions(ctx)
	waitForPayments(t, redsys, 1)
	require.Eventually(t, func() bool {
		r, _ := db.GetPaymentRetry(ctx, 5092)
		return r != nil
	}, 2*time.Second, 10*time.Millisecond)

	// As in production, the transaction keeps its own copy of the card, made
	// before the decline: fail count 0. (The mock otherwise shares one card
	// object between both, which would hide a stale copy.)
	tx, _ := db.GetTransaction(ctx, 5092)
	require.NotNil(t, tx.PaymentMethod)
	stale := *tx.PaymentMethod
	stale.FailCount = 0
	tx.PaymentMethod = &stale
	require.NoError(t, db.UpdateTransactionPayment(ctx, tx))

	// the user adds a card and starts again
	redsys.mu.Lock()
	redsys.decline = ""
	redsys.mu.Unlock()
	require.NoError(t, db.SavePaymentMethod(ctx, &entity.PaymentMethod{
		Identifier: "new-card", UserId: "uid-389b1", CofTid: "cof-2", ExpiryDate: "3012",
	}))
	require.NoError(t, c.validateStartTransactionPaymentMethod(ctx, startRequest))

	pays := redsys.payRequests()
	require.Len(t, pays, 2)
	// the transaction remembers the declined card with its old fail count of
	// 0; the retry must see the card's current state and switch
	assert.Equal(t, "new-card", pays[1].CardToken)
	tx, _ = db.GetTransaction(ctx, 5092)
	assert.Empty(t, tx.PaymentError)
	retry, _ := db.GetPaymentRetry(ctx, 5092)
	assert.Nil(t, retry, "paid: off the retry queue")
}

func TestPaymentInFlightIsNotSentTwice(t *testing.T) {
	c, db, redsys := unbilled(t, &entity.PaymentMethod{Identifier: "card", CofTid: "cof", ExpiryDate: "3012", IsDefault: true})
	ctx := context.Background()
	// an order opened a moment ago, its Redsys answer not in yet
	require.NoError(t, db.SavePaymentOrder(ctx, &entity.PaymentOrder{Order: 3150, TransactionId: 5092, Amount: 3024, TimeOpened: time.Now()}))

	require.NoError(t, c.PayTransaction(ctx, 5092))
	assert.Empty(t, redsys.payRequests())

	// an order left open for longer than any request runs is given up
	require.NoError(t, db.SavePaymentOrder(ctx, &entity.PaymentOrder{Order: 3150, TransactionId: 5092, Amount: 3024, TimeOpened: time.Now().Add(-10 * time.Minute)}))
	require.NoError(t, c.PayTransaction(ctx, 5092))
	waitForPayments(t, redsys, 1)
}

func TestEnrollmentSavesCard(t *testing.T) {
	tests := []struct {
		name  string
		saved *entity.PaymentMethod
	}{
		{name: "new card"},
		{
			name: "card already on file",
			saved: &entity.PaymentMethod{Identifier: "card", Description: "mine", IsDefault: true,
				CofTid: "cof-old", ExpiryDate: "2611", FailCount: 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := database_mock.NewMockDB()
			c := New(newTestLogger(), db)
			ctx := context.Background()
			if tt.saved != nil {
				tt.saved.UserId = "uid-158c0"
				require.NoError(t, db.SavePaymentMethod(ctx, tt.saved))
			}
			require.NoError(t, db.SavePaymentOrder(ctx, &entity.PaymentOrder{Order: 3184, UserId: "uid-158c0", UserName: "user_158c0"}))

			c.processPaymentResponse(ctx, &CaptureResponse{
				Success: true, ResponseCode: "0000", Amount: "0",
				MerchantIdentifier: "card", CofTxnid: "cof-new", CardBrand: "1", ExpiryDate: "3012",
			}, 3184)

			methods, _ := db.GetPaymentMethods(ctx, "uid-158c0")
			require.Len(t, methods, 1)
			pm := methods[0]
			assert.Equal(t, "cof-new", pm.CofTid)
			assert.Equal(t, "3012", pm.ExpiryDate)
			assert.Zero(t, pm.FailCount)
			if tt.saved != nil {
				assert.Equal(t, "mine", pm.Description)
				assert.True(t, pm.IsDefault)
			}
			assert.Equal(t, pm, c.pickFreshPaymentMethod(ctx, "uid-158c0", ""))
		})
	}
}

// --- old unpaid sessions are collected on a card that works ---

// withDebt adds transaction 3709 as it is in production: declined with 0190
// in February, four attempts, retries long exhausted, so it is marked billed
// and nothing will charge it again.
func withDebt(t *testing.T, db *database_mock.MockDB) {
	t.Helper()
	opened := time.Date(2026, 2, 23, 7, 46, 0, 0, time.UTC)
	db.SeedTransaction(&entity.Transaction{
		TransactionId: 3709, ChargePointId: "PE00004", ConnectorId: 1, IdTag: "TAG", IsFinished: true,
		MeterStop: 6141, PaymentAmount: 276, PaymentBilled: 276, PaymentError: "0190", PaymentOrder: 2763,
		TimeStop:      time.Date(2026, 2, 12, 18, 52, 20, 0, time.UTC),
		PaymentOrders: []entity.PaymentOrder{{Order: 2763, TransactionId: 3709, Amount: 276, IsCompleted: true, Result: "0190", TimeOpened: opened, TimeClosed: opened}},
		UserTag:       &entity.UserTag{IdTag: "TAG", UserId: "uid-389b1", Username: "user_389b1"},
	})
}

// debtor is user_389b1 with debt 3709, a new session 5092 not charged yet and
// one card on file that works.
func debtor(t *testing.T) (*Core, *database_mock.MockDB, *stubRedsys) {
	t.Helper()
	c, db, redsys := unbilled(t, &entity.PaymentMethod{Identifier: "new-card", CofTid: "cof", ExpiryDate: "3211", IsDefault: true})
	withTag(t, db)
	withDebt(t, db)
	return c, db, redsys
}

func waitForTransaction(t *testing.T, db *database_mock.MockDB, id int, done func(*entity.Transaction) bool) *entity.Transaction {
	t.Helper()
	var tx *entity.Transaction
	require.Eventually(t, func() bool {
		tx, _ = db.GetTransaction(context.Background(), id)
		return tx != nil && done(tx)
	}, 2*time.Second, 10*time.Millisecond)
	return tx
}

func paid(tx *entity.Transaction) bool {
	return tx.PaymentBilled >= tx.PaymentAmount && tx.PaymentError == ""
}

func TestCapturedPaymentCollectsOldDebt(t *testing.T) {
	c, db, redsys := debtor(t)
	ctx := context.Background()

	require.NoError(t, c.PayTransaction(ctx, 5092))

	pays := waitForPayments(t, redsys, 2)
	assert.Equal(t, 3024, pays[0].Amount, "the new session first")
	assert.Equal(t, 276, pays[1].Amount, "then the old debt")
	assert.Equal(t, "new-card", pays[1].CardToken, "on the card that has just been charged")

	tx := waitForTransaction(t, db, 3709, paid)
	assert.Equal(t, 276, tx.PaymentBilled)
	require.Len(t, tx.PaymentOrders, 2)
	assert.True(t, tx.PaymentOrders[1].Debt, "the order is marked as collecting a debt")

	// its capture looks for more; nothing is owed
	time.Sleep(100 * time.Millisecond)
	assert.Len(t, redsys.payRequests(), 2)
}

func TestDebtsCollectedOneByOne(t *testing.T) {
	c, db, redsys := debtor(t)
	ctx := context.Background()
	db.SeedTransaction(&entity.Transaction{
		TransactionId: 3800, ChargePointId: "PE00004", IdTag: "TAG", IsFinished: true,
		MeterStop: 2000, PaymentAmount: 90, PaymentBilled: 90, PaymentError: "SIS0334",
		TimeStop: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
	})

	require.NoError(t, c.PayTransaction(ctx, 5092))

	pays := waitForPayments(t, redsys, 3)
	assert.Equal(t, []int{3024, 276, 90}, []int{pays[0].Amount, pays[1].Amount, pays[2].Amount}, "oldest debt first")
	waitForTransaction(t, db, 3709, paid)
	waitForTransaction(t, db, 3800, paid)
}

// A decline of the old debt must not bar the card the user pays with now,
// nor put the debt back on the retry queue.
func TestDeclinedDebtDoesNotBlockCard(t *testing.T) {
	c, db, redsys := debtor(t)
	redsys.declines = []string{"", "0190"}
	ctx := context.Background()

	require.NoError(t, c.PayTransaction(ctx, 5092))
	waitForPayments(t, redsys, 2)

	tx := waitForTransaction(t, db, 3709, func(tx *entity.Transaction) bool { return tx.PaymentError != "" })
	assert.Equal(t, "0190", tx.PaymentError)
	assert.Equal(t, 276, tx.PaymentBilled, "marked billed again, as a declined payment")
	retry, _ := db.GetPaymentRetry(ctx, 3709)
	assert.Nil(t, retry, "an exhausted debt is not queued again")

	card, err := db.GetPaymentMethodByIdentifier(ctx, "new-card")
	require.NoError(t, err)
	assert.Zero(t, card.FailCount, "the card still starts sessions")
	require.NoError(t, c.validateStartTransactionPaymentMethod(ctx, startRequest))

	// the next payment on the same day does not ask for the debt again
	c.collectDebts(ctx, "uid-389b1", "new-card")
	time.Sleep(100 * time.Millisecond)
	assert.Len(t, redsys.payRequests(), 2)
}

func TestEnrollmentCollectsDebt(t *testing.T) {
	db := database_mock.NewMockDB()
	redsys := &stubRedsys{}
	c := New(newTestLogger(), db)
	c.SetRedsys(redsys)
	withTag(t, db)
	withDebt(t, db)
	ctx := context.Background()
	require.NoError(t, db.SavePaymentOrder(ctx, &entity.PaymentOrder{Order: 3104, UserId: "uid-389b1", UserName: "user_389b1"}))

	c.processPaymentResponse(ctx, &CaptureResponse{
		Success: true, ResponseCode: "0000", Amount: "0",
		MerchantIdentifier: "new-card", CofTxnid: "cof", ExpiryDate: "3211",
	}, 3104)

	req := waitForPayments(t, redsys, 1)[0]
	assert.Equal(t, "new-card", req.CardToken)
	assert.Equal(t, 276, req.Amount)
	waitForTransaction(t, db, 3709, paid)
}

func TestDebtNotCollectedOnUnusableCard(t *testing.T) {
	for _, card := range []*entity.PaymentMethod{
		{Identifier: "card", CofTid: "cof", ExpiryDate: "3211", FailCount: 1},
		{Identifier: "card", ExpiryDate: "3211"},
		{Identifier: "card", CofTid: "cof", ExpiryDate: "2001"},
	} {
		db := database_mock.NewMockDB()
		redsys := &stubRedsys{}
		c := New(newTestLogger(), db)
		c.SetRedsys(redsys)
		withTag(t, db)
		withDebt(t, db)
		card.UserId = "uid-389b1"
		require.NoError(t, db.SavePaymentMethod(context.Background(), card))

		c.collectDebts(context.Background(), "uid-389b1", "card")
		assert.Empty(t, redsys.payRequests(), "%+v", card)
		tx, _ := db.GetTransaction(context.Background(), 3709)
		assert.Equal(t, "0190", tx.PaymentError, "left as it was")
		assert.Equal(t, 276, tx.PaymentBilled)
	}
}

// Another user's card is never charged for this user's debt.
func TestDebtNotCollectedOnOtherUsersCard(t *testing.T) {
	c, db, redsys := unbilled(t)
	withTag(t, db)
	withDebt(t, db)
	require.NoError(t, db.SavePaymentMethod(context.Background(),
		&entity.PaymentMethod{Identifier: "theirs", UserId: "uid-other", CofTid: "cof", ExpiryDate: "3211"}))

	c.collectDebts(context.Background(), "uid-389b1", "theirs")
	assert.Empty(t, redsys.payRequests())
}
