package entity

// TransactionMail carries everything the transaction email renders: the stored
// transaction plus the charge point descriptors, which live in a separate
// collection and are not embedded in the transaction document.
type TransactionMail struct {
	Transaction        *Transaction
	ChargePointTitle   string
	ChargePointAddress string
}
