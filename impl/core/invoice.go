package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"evsys-back/entity"
	"evsys-back/impl/holded"
	"evsys-back/internal/lib/sl"
)

// maxInvoiceAttempts is how many times a job is tried before it is given up.
const maxInvoiceAttempts = 6

// invoiceDelays is the wait before the next try, by the number of failures.
var invoiceDelays = []time.Duration{
	time.Minute,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	12 * time.Hour,
	24 * time.Hour,
}

// unauthorizedDelay is the pause after Holded rejects the key. The failure is
// not the job's fault, so the attempt is not counted.
const unauthorizedDelay = time.Hour

// InvoiceClient is the part of the Holded client the invoice queue uses.
type InvoiceClient interface {
	CreateContact(ctx context.Context, contact holded.Contact) (string, error)
	CreateInvoice(ctx context.Context, invoice holded.Invoice) (string, error)
	ApproveInvoice(ctx context.Context, id string) error
	AddPayment(ctx context.Context, id string, payment holded.Payment) error
}

// InvoiceSettings are the Holded document settings kept in the config. They
// are left empty until the accountant decides (task P-44): an empty Tax issues
// a line without tax, an empty NumberLineId uses the default series.
type InvoiceSettings struct {
	Tax          string // tax code of the line, e.g. "s_iva_21"
	NumberLineId string // numbering series
	TreasuryId   string // account the payment is registered on
	Description  string // line text; the transaction number is appended
}

// SetInvoiceClient turns the invoice queue on. Without a client nothing is
// queued and nothing is issued.
func (c *Core) SetInvoiceClient(client InvoiceClient, settings InvoiceSettings) {
	c.invoices = client
	c.invoiceSettings = settings
}

// invoiceError is what core needs to know about a failed Holded call; it is
// satisfied by *holded.Error.
type invoiceError interface {
	error
	Retryable() bool
	Ambiguous() bool
	Unauthorized() bool
	Wait() time.Duration
}

// enqueueInvoice queues an invoice for a session that has just been paid in
// full. It is called from the payment flow and must never fail it: every
// problem is logged and swallowed. A session is invoiced once - the queue has
// a unique key on the transaction id - and only if its user asked for
// invoices.
func (c *Core) enqueueInvoice(ctx context.Context, transaction *entity.Transaction, userId string) {
	if c.invoices == nil || userId == "" {
		return
	}
	// PaymentBilled alone is no proof of payment (failed sessions get it set
	// too); this runs only on a successful Redsys answer. Several orders can
	// pay one session: wait for the last one.
	if transaction.PaymentAmount <= 0 || transaction.PaymentBilled < transaction.PaymentAmount {
		return
	}
	log := c.log.With(slog.Int("transaction_id", transaction.TransactionId))
	profile, err := c.repo.GetInvoiceProfile(ctx, userId)
	if err != nil {
		log.With(sl.Err(err)).Error("invoice: failed to read the profile")
		return
	}
	if profile == nil || !profile.Enabled || profile.ConsentAt == nil {
		return
	}
	now := time.Now()
	job := &entity.InvoiceJob{
		TransactionId:   transaction.TransactionId,
		UserId:          userId,
		Amount:          transaction.PaymentBilled,
		Status:          entity.InvoiceJobPending,
		NextAttemptTime: now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	created, err := c.repo.CreateInvoiceJob(ctx, job)
	if err != nil {
		log.With(sl.Err(err)).Error("invoice: failed to queue")
		return
	}
	if created {
		c.payLog(ctx, "info", "invoice", "transaction %d: invoice queued (user %s)", transaction.TransactionId, userId)
	}
}

// processInvoices runs the jobs whose turn has come, one after another: the
// pace stays well under Holded's rate limits and a contact is never created
// twice by two jobs of one user.
func (c *Core) processInvoices(ctx context.Context) {
	if c.invoices == nil {
		return
	}
	jobs, err := c.repo.GetPendingInvoiceJobs(ctx, time.Now())
	if err != nil {
		c.log.With(sl.Err(err)).Error("invoice: failed to read the queue")
		return
	}
	for _, job := range jobs {
		if ctx.Err() != nil {
			return
		}
		if _, held := c.invoiceUnsaved.Load(job.TransactionId); held {
			// Holded was changed but the progress was not saved: do not
			// risk a second document
			continue
		}
		c.guardInvoice(ctx, job)
	}
}

// guardInvoice runs one job and turns a panic into a failed attempt: the
// processor runs on a bare goroutine, and a job that panics would otherwise
// bring the backend down again on every pass.
func (c *Core) guardInvoice(ctx context.Context, job *entity.InvoiceJob) {
	defer func() {
		if r := recover(); r != nil {
			c.log.With(
				slog.Int("transaction_id", job.TransactionId),
				slog.Any("panic", r),
				slog.String("stack", string(debug.Stack())),
			).Error("invoice processing panicked")
			c.failInvoice(ctx, job, fmt.Errorf("panic: %v", r), false)
		}
	}()
	c.runInvoice(ctx, job)
}

// runInvoice moves a job through contact, create, approve, payment. Progress
// is saved after every call that changes Holded, so a retry never repeats a
// step that is done.
func (c *Core) runInvoice(ctx context.Context, job *entity.InvoiceJob) {
	profile, err := c.repo.GetInvoiceProfile(ctx, job.UserId)
	if err != nil {
		c.failInvoice(ctx, job, fmt.Errorf("read profile: %w", err), false)
		return
	}
	if profile == nil || !profile.Enabled {
		// the user turned invoices off or deleted the data after paying
		if job.Step == entity.InvoiceStepNew {
			c.finishInvoice(ctx, job, entity.InvoiceJobSkipped, "profile turned off or removed")
			return
		}
		// a document is already in Holded: finish it, the profile is not needed
	}

	if job.Step == entity.InvoiceStepNew {
		contactId, err := c.invoiceContact(ctx, profile)
		if err != nil {
			c.failInvoice(ctx, job, err, false)
			return
		}
		id, err := c.invoices.CreateInvoice(ctx, c.holdedInvoice(job, contactId))
		if err != nil {
			// the document may exist although the call failed
			c.failInvoice(ctx, job, err, true)
			return
		}
		job.HoldedInvoiceId = id
		job.Step = entity.InvoiceStepCreated
		if !c.saveInvoiceJob(ctx, job) {
			// the id could not be saved: stop, the job is held back
			return
		}
	}

	if job.Step == entity.InvoiceStepCreated {
		if err := c.invoices.ApproveInvoice(ctx, job.HoldedInvoiceId); err != nil {
			c.failInvoice(ctx, job, err, false)
			return
		}
		job.Step = entity.InvoiceStepApproved
		if !c.saveInvoiceJob(ctx, job) {
			return
		}
	}

	if job.Step == entity.InvoiceStepApproved {
		payment := holded.Payment{
			Amount:     holded.Euros(job.Amount),
			TreasuryId: c.invoiceSettings.TreasuryId,
			Date:       time.Now().UTC().Format("2006-01-02"),
		}
		if err := c.invoices.AddPayment(ctx, job.HoldedInvoiceId, payment); err != nil {
			// a repeated payment would be registered twice
			c.failInvoice(ctx, job, err, true)
			return
		}
		job.Step = entity.InvoiceStepPaid
	}

	c.finishInvoice(ctx, job, entity.InvoiceJobDone, "")
	c.payLog(ctx, "info", "invoice", "transaction %d: invoice %s issued", job.TransactionId, job.HoldedInvoiceId)
}

// invoiceContact returns the customer's Holded contact, creating it on the
// first invoice and keeping its id on the profile.
func (c *Core) invoiceContact(ctx context.Context, profile *entity.InvoiceProfile) (string, error) {
	if profile.HoldedContactId != "" {
		return profile.HoldedContactId, nil
	}
	id, err := c.invoices.CreateContact(ctx, holded.Contact{
		Name:     profile.Name,
		VatId:    profile.TaxId,
		IsPerson: profile.Kind != entity.InvoiceKindCompany,
		Email:    profile.Email,
		CustomId: profile.UserId,
		Address: holded.Address{
			Address:     profile.Address,
			City:        profile.City,
			PostalCode:  profile.PostalCode,
			Province:    profile.Province,
			CountryCode: profile.CountryCode,
		},
	})
	if err != nil {
		return "", err
	}
	if err := c.repo.SetInvoiceProfileContact(ctx, profile.UserId, id); err != nil {
		// the contact exists in Holded: not an error of the job, but the next
		// job of this user would create another one
		c.log.With(sl.Err(err), slog.String("contact", id)).
			Error("invoice: contact created but its id was not saved")
	}
	return id, nil
}

// holdedInvoice builds the document. Amounts come from the server (cents of
// EUR). Whether Holded takes the price with or without tax is not settled:
// with a tax code set the total can differ from the charge, so the first
// pilot invoice must be compared to the payment to the cent.
func (c *Core) holdedInvoice(job *entity.InvoiceJob, contactId string) holded.Invoice {
	item := holded.Item{
		Name:  fmt.Sprintf("%s #%d", c.invoiceDescription(), job.TransactionId),
		Units: 1,
		Price: holded.Euros(job.Amount),
	}
	if c.invoiceSettings.Tax != "" {
		item.Taxes = []string{c.invoiceSettings.Tax}
	}
	return holded.Invoice{
		ContactId:    contactId,
		Items:        []holded.Item{item},
		Date:         time.Now().UTC().Format("2006-01-02"),
		NumberLineId: c.invoiceSettings.NumberLineId,
		CustomFields: map[string]any{"transaction_id": job.TransactionId},
	}
}

func (c *Core) invoiceDescription() string {
	if c.invoiceSettings.Description != "" {
		return c.invoiceSettings.Description
	}
	return "EV charging session"
}

// failInvoice records a failed attempt and decides what happens next.
// creating marks a call that creates a document: if it is ambiguous, the job
// stops for a manual check instead of being repeated.
func (c *Core) failInvoice(ctx context.Context, job *entity.InvoiceJob, err error, creating bool) {
	log := c.log.With(slog.Int("transaction_id", job.TransactionId), sl.Err(err))
	now := time.Now()
	job.LastError = err.Error()
	job.UpdatedAt = now

	var he invoiceError
	known := errors.As(err, &he)
	switch {
	case known && he.Unauthorized():
		// the key or its rights: the job is fine, the operator must act
		job.NextAttemptTime = now.Add(unauthorizedDelay)
		log.Error("invoice: Holded rejected the key")
		c.payLog(ctx, "error", "invoice", "transaction %d: Holded rejected the API key or its permissions: %v", job.TransactionId, err)
	case known && creating && he.Ambiguous():
		job.Status = entity.InvoiceJobNeedsCheck
		log.Error("invoice: the document may exist in Holded, job stopped")
		c.payLog(ctx, "error", "invoice",
			"transaction %d: no reliable answer from Holded, the document may exist (step %q, invoice %q); check the account before repeating: %v",
			job.TransactionId, job.Step, job.HoldedInvoiceId, err)
	case known && !he.Retryable():
		job.Status = entity.InvoiceJobFailed
		log.Error("invoice: Holded refused")
		c.payLog(ctx, "error", "invoice", "transaction %d: Holded refused the invoice: %v", job.TransactionId, err)
	default:
		job.Attempt++
		if job.Attempt >= maxInvoiceAttempts {
			job.Status = entity.InvoiceJobFailed
			log.Error("invoice: retries exhausted")
			c.payLog(ctx, "error", "invoice", "transaction %d: invoice failed after %d attempts: %v", job.TransactionId, job.Attempt, err)
			break
		}
		delay := invoiceDelays[min(job.Attempt-1, len(invoiceDelays)-1)]
		if known && he.Wait() > delay {
			delay = he.Wait()
		}
		job.NextAttemptTime = now.Add(delay)
		log.Warn("invoice: attempt failed, will retry", slog.Int("attempt", job.Attempt))
	}
	c.saveInvoiceJob(ctx, job)
}

// finishInvoice closes a job.
func (c *Core) finishInvoice(ctx context.Context, job *entity.InvoiceJob, status, note string) {
	job.Status = status
	job.LastError = note
	job.UpdatedAt = time.Now()
	c.saveInvoiceJob(ctx, job)
}

// saveInvoiceJob stores the job. If that fails after Holded was changed, the
// stored job still shows the old step and the next pass would repeat a
// creating call. The job is therefore held back in memory until a save works
// (a restart loses that, but the transaction id is on the Holded document), and
// the failure is reported loudly.
func (c *Core) saveInvoiceJob(ctx context.Context, job *entity.InvoiceJob) bool {
	job.UpdatedAt = time.Now()
	err := c.repo.UpdateInvoiceJob(ctx, job)
	if err == nil {
		c.invoiceUnsaved.Delete(job.TransactionId)
		return true
	}
	c.invoiceUnsaved.Store(job.TransactionId, job.HoldedInvoiceId)
	c.log.With(sl.Err(err), slog.Int("transaction_id", job.TransactionId),
		slog.String("holded_invoice", job.HoldedInvoiceId), slog.String("step", job.Step)).
		Error("invoice: failed to save job progress")
	c.payLog(ctx, "error", "invoice",
		"transaction %d: could not save progress (step %q, Holded invoice %q): %v",
		job.TransactionId, job.Step, job.HoldedInvoiceId, err)
	return false
}

// ListInvoiceJobs returns the jobs that need an operator (failed, stopped for
// a check) and, with all, the rest too. Admin only.
func (c *Core) ListInvoiceJobs(ctx context.Context, author *entity.User, all bool) ([]*entity.InvoiceJob, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	if all {
		return c.repo.ListInvoiceJobs(ctx)
	}
	return c.repo.ListInvoiceJobs(ctx, entity.InvoiceJobFailed, entity.InvoiceJobNeedsCheck)
}
