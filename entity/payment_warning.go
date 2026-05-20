package entity

import "time"

// PaymentWarning carries the details of a failed payment attempt, used to
// build the operator warning email.
type PaymentWarning struct {
	TransactionId int
	OrderNumber   int
	ChargePointId string
	CardHolder    string
	Amount        int    // minor currency units
	Currency      string // Redsys numeric currency code, may be empty
	Result        string // Redsys response/error code or message
	Attempt       int    // retry attempt number (1-based); 0 when no retry applies
	MaxAttempts   int
	Exhausted     bool // true when no further retry is scheduled
	OccurredAt    time.Time
}
