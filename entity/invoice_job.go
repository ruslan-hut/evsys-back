package entity

import "time"

// Invoice job statuses.
const (
	// InvoiceJobPending waits for its turn (NextAttemptTime) in the queue.
	InvoiceJobPending = "pending"
	// InvoiceJobDone: the invoice is issued and its payment registered.
	InvoiceJobDone = "done"
	// InvoiceJobFailed: Holded refused the data or the retries ran out. An
	// operator has to look at LastError.
	InvoiceJobFailed = "failed"
	// InvoiceJobNeedsCheck: a call that creates something in Holded got no
	// reliable answer, so the document may exist. Holded has no idempotency
	// key, so the job stops until somebody compares with the Holded account.
	InvoiceJobNeedsCheck = "needs_check"
	// InvoiceJobSkipped: nothing to issue (the user turned invoices off or
	// removed the profile before the job ran).
	InvoiceJobSkipped = "skipped"
)

// Invoice job steps, in order. Step is the last step that finished: a retry
// continues after it and never repeats a call that already succeeded.
const (
	InvoiceStepNew      = ""
	InvoiceStepCreated  = "created"  // invoice exists in Holded, HoldedInvoiceId is saved
	InvoiceStepApproved = "approved" // draft turned into an issued invoice
	InvoiceStepPaid     = "paid"     // payment registered on the invoice
)

// InvoiceJob is one invoice to issue, queued after a session is paid.
// There is one job per transaction (unique index on transaction_id). Money is
// in cents of EUR, taken from the transaction on the server.
type InvoiceJob struct {
	TransactionId   int       `json:"transaction_id" bson:"transaction_id"`
	UserId          string    `json:"user_id" bson:"user_id"`
	Amount          int       `json:"amount" bson:"amount"`
	Status          string    `json:"status" bson:"status"`
	Step            string    `json:"step" bson:"step"`
	HoldedInvoiceId string    `json:"holded_invoice_id,omitempty" bson:"holded_invoice_id,omitempty"`
	Attempt         int       `json:"attempt" bson:"attempt"`
	NextAttemptTime time.Time `json:"next_attempt_time" bson:"next_attempt_time"`
	LastError       string    `json:"last_error,omitempty" bson:"last_error,omitempty"`
	CreatedAt       time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" bson:"updated_at"`
}
