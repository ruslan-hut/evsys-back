package core

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"evsys-back/entity"
	"evsys-back/impl/holded"

	database_mock "evsys-back/impl/database-mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeHolded records the calls and fails the ones listed in errs by name.
type fakeHolded struct {
	mu       sync.Mutex
	calls    []string
	invoices []holded.Invoice
	contacts []holded.Contact
	payments []holded.Payment
	errs     map[string]error
}

func (f *fakeHolded) call(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, name)
	return f.errs[name]
}

func (f *fakeHolded) CreateContact(_ context.Context, c holded.Contact) (string, error) {
	if err := f.call("contact"); err != nil {
		return "", err
	}
	f.contacts = append(f.contacts, c)
	return "contact-1", nil
}

func (f *fakeHolded) CreateInvoice(_ context.Context, i holded.Invoice) (string, error) {
	if err := f.call("invoice"); err != nil {
		return "", err
	}
	f.invoices = append(f.invoices, i)
	return "inv-1", nil
}

func (f *fakeHolded) ApproveInvoice(context.Context, string) error { return f.call("approve") }

func (f *fakeHolded) AddPayment(_ context.Context, _ string, p holded.Payment) error {
	if err := f.call("payment"); err != nil {
		return err
	}
	f.payments = append(f.payments, p)
	return nil
}

func (f *fakeHolded) names() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func invoiceSetup(t *testing.T, errs map[string]error) (*Core, *database_mock.MockDB, *fakeHolded) {
	t.Helper()
	db := database_mock.NewMockDB()
	fake := &fakeHolded{errs: errs}
	c := New(newTestLogger(), db)
	c.SetInvoiceClient(fake, InvoiceSettings{Tax: "s_iva_21", NumberLineId: "serie-1", TreasuryId: "bank"})
	now := time.Now()
	require.NoError(t, db.SaveInvoiceProfile(context.Background(), &entity.InvoiceProfile{
		UserId: "ana", Enabled: true, Kind: entity.InvoiceKindPerson, Name: "Ana", TaxId: "12345678Z",
		Address: "C. Cuenca 1", City: "El Pedernoso", PostalCode: "16690", CountryCode: "ES",
		ConsentAt: &now, UpdatedAt: now,
	}))
	return c, db, fake
}

func paidTx(billed int) *entity.Transaction {
	return &entity.Transaction{TransactionId: 7001, PaymentAmount: 3024, PaymentBilled: billed}
}

func jobOf(t *testing.T, db *database_mock.MockDB) *entity.InvoiceJob {
	t.Helper()
	jobs, err := db.ListInvoiceJobs(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	return jobs[0]
}

func TestInvoiceHappyPath(t *testing.T) {
	ctx := context.Background()
	c, db, fake := invoiceSetup(t, nil)

	c.enqueueInvoice(ctx, paidTx(3024), "ana")
	c.enqueueInvoice(ctx, paidTx(3024), "ana") // a repeat does not queue a second job
	c.processInvoices(ctx)

	assert.Equal(t, []string{"contact", "invoice", "approve", "payment"}, fake.names())
	job := jobOf(t, db)
	assert.Equal(t, entity.InvoiceJobDone, job.Status)
	assert.Equal(t, "inv-1", job.HoldedInvoiceId)

	inv := fake.invoices[0]
	assert.Equal(t, "contact-1", inv.ContactId)
	assert.Equal(t, "serie-1", inv.NumberLineId)
	assert.Equal(t, 30.24, inv.Items[0].Price)
	assert.Equal(t, []string{"s_iva_21"}, inv.Items[0].Taxes)
	assert.Equal(t, 7001, inv.CustomFields["transaction_id"])
	assert.Equal(t, 30.24, fake.payments[0].Amount)
	assert.Equal(t, "bank", fake.payments[0].TreasuryId)

	// the contact is kept on the profile: the next invoice does not create another
	p, _ := db.GetInvoiceProfile(ctx, "ana")
	assert.Equal(t, "contact-1", p.HoldedContactId)
	assert.Equal(t, "12345678Z", fake.contacts[0].VatId)

	// a finished job is not picked up again
	c.processInvoices(ctx)
	assert.Len(t, fake.names(), 4)
}

func TestInvoiceNotQueued(t *testing.T) {
	ctx := context.Background()
	c, db, _ := invoiceSetup(t, nil)

	c.enqueueInvoice(ctx, paidTx(1000), "ana") // only part of the amount is paid
	c.enqueueInvoice(ctx, &entity.Transaction{TransactionId: 7002}, "ana")
	c.enqueueInvoice(ctx, paidTx(3024), "")    // roaming, no app user
	c.enqueueInvoice(ctx, paidTx(3024), "bob") // no profile

	off, _ := db.GetInvoiceProfile(ctx, "ana")
	off.Enabled = false
	require.NoError(t, db.SaveInvoiceProfile(ctx, off))
	c.enqueueInvoice(ctx, paidTx(3024), "ana") // invoices turned off

	jobs, _ := db.ListInvoiceJobs(ctx)
	assert.Empty(t, jobs)

	// no client: nothing is queued
	c2 := New(newTestLogger(), db)
	c2.enqueueInvoice(ctx, paidTx(3024), "ana")
	jobs, _ = db.ListInvoiceJobs(ctx)
	assert.Empty(t, jobs)
}

func TestInvoiceSkippedWhenProfileTurnedOff(t *testing.T) {
	ctx := context.Background()
	c, db, fake := invoiceSetup(t, nil)
	c.enqueueInvoice(ctx, paidTx(3024), "ana")

	p, _ := db.GetInvoiceProfile(ctx, "ana")
	p.Enabled = false
	require.NoError(t, db.SaveInvoiceProfile(ctx, p))
	c.processInvoices(ctx)

	assert.Empty(t, fake.names())
	assert.Equal(t, entity.InvoiceJobSkipped, jobOf(t, db).Status)
}

func TestInvoiceRetryContinuesAfterDoneStep(t *testing.T) {
	ctx := context.Background()
	c, db, fake := invoiceSetup(t, map[string]error{
		"approve": &holded.Error{Op: "approve invoice", Status: http.StatusTooManyRequests, RetryAfter: 3 * time.Hour},
	})
	c.enqueueInvoice(ctx, paidTx(3024), "ana")
	c.processInvoices(ctx)

	job := jobOf(t, db)
	assert.Equal(t, entity.InvoiceJobPending, job.Status)
	assert.Equal(t, entity.InvoiceStepCreated, job.Step)
	assert.Equal(t, 1, job.Attempt)
	assert.WithinDuration(t, time.Now().Add(3*time.Hour), job.NextAttemptTime, time.Minute) // Retry-After wins

	// not due yet: nothing happens
	c.processInvoices(ctx)
	assert.Len(t, fake.names(), 3)

	// due: continues with approve, the invoice is not created again
	fake.errs = nil
	job.NextAttemptTime = time.Now().Add(-time.Second)
	require.NoError(t, db.UpdateInvoiceJob(ctx, job))
	c.processInvoices(ctx)

	assert.Equal(t, []string{"contact", "invoice", "approve", "approve", "payment"}, fake.names())
	assert.Equal(t, entity.InvoiceJobDone, jobOf(t, db).Status)
}

func TestInvoiceAmbiguousCreateStopsForCheck(t *testing.T) {
	ctx := context.Background()
	for name, err := range map[string]error{
		"no answer":    &holded.Error{Op: "create invoice", Err: errors.New("timeout")},
		"server error": &holded.Error{Op: "create invoice", Status: http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			c, db, fake := invoiceSetup(t, map[string]error{"invoice": err})
			c.enqueueInvoice(ctx, paidTx(3024), "ana")
			c.processInvoices(ctx)
			c.processInvoices(ctx)

			assert.Equal(t, entity.InvoiceJobNeedsCheck, jobOf(t, db).Status)
			assert.Equal(t, []string{"contact", "invoice"}, fake.names(), "never repeated blindly")
		})
	}
}

func TestInvoiceAmbiguousPaymentStopsForCheck(t *testing.T) {
	ctx := context.Background()
	c, db, _ := invoiceSetup(t, map[string]error{"payment": &holded.Error{Op: "add payment", Status: 503}})
	c.enqueueInvoice(ctx, paidTx(3024), "ana")
	c.processInvoices(ctx)

	job := jobOf(t, db)
	assert.Equal(t, entity.InvoiceJobNeedsCheck, job.Status)
	assert.Equal(t, entity.InvoiceStepApproved, job.Step)
}

func TestInvoiceRefusedAndExhausted(t *testing.T) {
	ctx := context.Background()

	// a 422 is the data's fault: no retry
	c, db, fake := invoiceSetup(t, map[string]error{"contact": &holded.Error{Op: "create contact", Status: 422, Body: "bad vat"}})
	c.enqueueInvoice(ctx, paidTx(3024), "ana")
	c.processInvoices(ctx)
	assert.Equal(t, entity.InvoiceJobFailed, jobOf(t, db).Status)
	assert.Contains(t, jobOf(t, db).LastError, "bad vat")
	assert.Len(t, fake.names(), 1)

	// a 503 on approve is retried until the attempts run out
	c, db, _ = invoiceSetup(t, map[string]error{"approve": &holded.Error{Op: "approve invoice", Status: 503}})
	c.enqueueInvoice(ctx, paidTx(3024), "ana")
	for i := 0; i < maxInvoiceAttempts; i++ {
		job := jobOf(t, db)
		job.NextAttemptTime = time.Now().Add(-time.Second)
		require.NoError(t, db.UpdateInvoiceJob(ctx, job))
		c.processInvoices(ctx)
	}
	job := jobOf(t, db)
	assert.Equal(t, entity.InvoiceJobFailed, job.Status)
	assert.Equal(t, maxInvoiceAttempts, job.Attempt)
}

func TestInvoiceUnauthorizedDoesNotBurnAttempts(t *testing.T) {
	ctx := context.Background()
	c, db, _ := invoiceSetup(t, map[string]error{"contact": &holded.Error{Op: "create contact", Status: http.StatusUnauthorized}})
	c.enqueueInvoice(ctx, paidTx(3024), "ana")
	c.processInvoices(ctx)

	job := jobOf(t, db)
	assert.Equal(t, entity.InvoiceJobPending, job.Status)
	assert.Equal(t, 0, job.Attempt)
	assert.WithinDuration(t, time.Now().Add(unauthorizedDelay), job.NextAttemptTime, time.Minute)
}

func TestInvoiceQueuedAfterSuccessfulPayment(t *testing.T) {
	ctx := context.Background()
	c, db, _ := invoiceSetup(t, nil)
	db.SeedTransaction(&entity.Transaction{TransactionId: 7001, ChargePointId: "PE00001", IdTag: "TAG", IsFinished: true, PaymentAmount: 3024})
	require.NoError(t, db.SavePaymentOrder(ctx, &entity.PaymentOrder{Order: 4001, TransactionId: 7001, UserId: "ana", UserName: "ana", Identifier: "card"}))

	c.processPaymentResponse(ctx, &CaptureResponse{Success: true, ResponseCode: "0000", Amount: "3024"}, 4001)

	job := jobOf(t, db)
	assert.Equal(t, 7001, job.TransactionId)
	assert.Equal(t, 3024, job.Amount)
	assert.Equal(t, entity.InvoiceJobPending, job.Status)
}

func TestInvoicePanicDoesNotKillProcessor(t *testing.T) {
	ctx := context.Background()
	c, db, _ := invoiceSetup(t, nil)
	c.invoices = &panicHolded{}
	c.enqueueInvoice(ctx, paidTx(3024), "ana")

	require.NotPanics(t, func() { c.processInvoices(ctx) })
	assert.Equal(t, 1, jobOf(t, db).Attempt)
}

type panicHolded struct{ fakeHolded }

func (*panicHolded) CreateContact(context.Context, holded.Contact) (string, error) { panic("boom") }
