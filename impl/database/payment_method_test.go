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
