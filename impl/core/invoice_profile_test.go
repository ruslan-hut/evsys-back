package core

import (
	"context"
	"errors"
	"evsys-back/entity"
	database_mock "evsys-back/impl/database-mock"
	"testing"
)

func TestInvoiceProfile(t *testing.T) {
	ctx := context.Background()
	c := New(newTestLogger(), database_mock.NewMockDB())
	ana := &entity.User{UserId: "ana", Username: "ana"}
	bob := &entity.User{UserId: "bob", Username: "bob"}

	// a user without a profile gets an empty one, invoices off
	p, err := c.GetInvoiceProfile(ctx, ana)
	if err != nil || p == nil || p.Enabled {
		t.Fatalf("empty profile: %+v %v", p, err)
	}

	req := &entity.InvoiceProfileRequest{Enabled: true, Consent: true, Kind: entity.InvoiceKindPerson,
		Name: "Ana", TaxId: "12345678Z", Address: "C. Cuenca 1", City: "El Pedernoso", PostalCode: "16690", CountryCode: "ES"}
	saved, err := c.SaveInvoiceProfile(ctx, ana, req)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ConsentAt == nil || !saved.Enabled || saved.UserId != "ana" {
		t.Fatalf("saved: %+v", saved)
	}
	consent := *saved.ConsentAt

	// a second save keeps the date of the first consent
	req.City = "Cuenca"
	saved, err = c.SaveInvoiceProfile(ctx, ana, req)
	if err != nil || saved.ConsentAt == nil || !saved.ConsentAt.Equal(consent) {
		t.Fatalf("consent date changed: %+v %v", saved, err)
	}

	// turning off keeps data and the consent date
	off := *req
	off.Enabled = false
	saved, err = c.SaveInvoiceProfile(ctx, ana, &off)
	if err != nil || saved.Enabled || saved.ConsentAt == nil || saved.City != "Cuenca" {
		t.Fatalf("after turning off: %+v %v", saved, err)
	}

	// profiles are per user
	other, err := c.GetInvoiceProfile(ctx, bob)
	if err != nil || other.Name != "" || other.Enabled {
		t.Fatalf("bob sees %+v %v", other, err)
	}

	// enabling without consent on a fresh profile leaves no consent date
	fresh, err := c.SaveInvoiceProfile(ctx, bob, &entity.InvoiceProfileRequest{Enabled: true, Name: "Bob"})
	if err != nil || fresh.ConsentAt != nil {
		t.Fatalf("consent date without consent: %+v %v", fresh, err)
	}

	// delete removes everything, a second delete is not found
	if err = c.DeleteInvoiceProfile(ctx, ana); err != nil {
		t.Fatal(err)
	}
	if err = c.DeleteInvoiceProfile(ctx, ana); !errors.Is(err, entity.ErrNotFound) {
		t.Errorf("second delete: %v, want ErrNotFound", err)
	}
	p, _ = c.GetInvoiceProfile(ctx, ana)
	if p.Name != "" {
		t.Errorf("profile survived delete: %+v", p)
	}
}
