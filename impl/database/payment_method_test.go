package database

import (
	"context"
	"errors"
	"evsys-back/entity"
	"testing"
)

// Run as described in concurrency_test.go.

func TestRenewPaymentMethod(t *testing.T) {
	m := testMongo(t)
	ctx := context.Background()

	saved := &entity.PaymentMethod{Identifier: "card", UserId: "uid-158c0", Description: "mine", IsDefault: true,
		CofTid: "cof-old", CardBrand: "1", ExpiryDate: "2611", FailCount: 3}
	if err := m.SavePaymentMethod(ctx, saved); err != nil {
		t.Fatal(err)
	}

	enrolled := &entity.PaymentMethod{Identifier: "card", UserId: "uid-158c0", Description: "**** **** **** ****",
		CofTid: "cof-new", CardBrand: "2", CardCountry: "724", ExpiryDate: "3012"}
	if err := m.RenewPaymentMethod(ctx, enrolled); err != nil {
		t.Fatal(err)
	}

	pm, err := m.GetPaymentMethod(ctx, "card", "uid-158c0")
	if err != nil || pm == nil {
		t.Fatalf("payment method: %v %v", pm, err)
	}
	if pm.CofTid != "cof-new" || pm.CardBrand != "2" || pm.CardCountry != "724" || pm.ExpiryDate != "3012" {
		t.Errorf("card data not renewed: %+v", pm)
	}
	if pm.FailCount != 0 {
		t.Errorf("fail count %d, want 0", pm.FailCount)
	}
	if pm.Description != "mine" || !pm.IsDefault {
		t.Errorf("user fields overwritten: %+v", pm)
	}

	// the same card token of another user is not touched
	other := &entity.PaymentMethod{Identifier: "card", UserId: "uid-other"}
	if err = m.RenewPaymentMethod(ctx, other); !errors.Is(err, entity.ErrNotFound) {
		t.Errorf("renew of a card not on file: %v, want ErrNotFound", err)
	}
}

func TestReopenFailedPayment(t *testing.T) {
	m := testMongo(t)
	ctx := context.Background()
	seed(t, m, collectionTransactions,
		entity.Transaction{TransactionId: 3709, IsFinished: true, PaymentAmount: 276, PaymentBilled: 276, PaymentError: "0190"},
		entity.Transaction{TransactionId: 5293, IsFinished: true, PaymentAmount: 414, PaymentBilled: 414},
		entity.Transaction{TransactionId: 5300, IsFinished: true, PaymentAmount: 300, PaymentBilled: 0, PaymentError: "0190"},
	)

	tests := []struct {
		name  string
		id    int
		error string
		want  bool
	}{
		{"error changed since the lookup", 3709, "SIS0334", false},
		{"paid", 5293, "", false},
		{"already reopened", 5300, "0190", false},
		{"failed payment", 3709, "0190", true},
		{"second time: it is reopened already", 3709, "0190", false},
	}
	for _, tt := range tests {
		got, err := m.ReopenFailedPayment(ctx, tt.id, tt.error)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got != tt.want {
			t.Errorf("%s: reopened %v, want %v", tt.name, got, tt.want)
		}
	}

	tx, err := m.GetTransaction(ctx, 3709)
	if err != nil || tx == nil {
		t.Fatalf("transaction: %v %v", tx, err)
	}
	if tx.PaymentBilled != 0 || tx.PaymentError != "" || tx.PaymentAmount != 276 {
		t.Errorf("reopened transaction: billed %d error %q amount %d", tx.PaymentBilled, tx.PaymentError, tx.PaymentAmount)
	}
}
