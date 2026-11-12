package database

import (
	"context"
	"errors"
	"evsys-back/entity"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// Run as described in concurrency_test.go.

func TestInvoiceProfile(t *testing.T) {
	m := testMongo(t)
	ctx := context.Background()

	if got, err := m.GetInvoiceProfile(ctx, "ana"); got != nil || err != nil {
		t.Fatalf("missing profile: %v %v, want nil, nil", got, err)
	}

	t1 := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	consent := t1
	p := &entity.InvoiceProfile{UserId: "ana", Enabled: true, Kind: entity.InvoiceKindPerson, Name: "Ana",
		TaxId: "12345678Z", Address: "C. Cuenca 1", City: "El Pedernoso", PostalCode: "16690",
		CountryCode: "ES", ConsentAt: &consent, UpdatedAt: t1}
	if err := m.SaveInvoiceProfile(ctx, p); err != nil {
		t.Fatal(err)
	}
	// another user's profile must not be touched
	if err := m.SaveInvoiceProfile(ctx, &entity.InvoiceProfile{UserId: "bob", Name: "Bob", UpdatedAt: t1}); err != nil {
		t.Fatal(err)
	}

	// the server sets the Holded contact; a later save must not wipe it
	_, err := m.col(collectionInvoiceProfiles).UpdateOne(ctx, bson.D{{Key: "user_id", Value: "ana"}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "holded_contact_id", Value: "h-1"}}}})
	if err != nil {
		t.Fatal(err)
	}

	t2 := t1.Add(time.Hour)
	p.City = "Cuenca"
	p.UpdatedAt = t2
	if err = m.SaveInvoiceProfile(ctx, p); err != nil {
		t.Fatal(err)
	}

	got, err := m.GetInvoiceProfile(ctx, "ana")
	if err != nil {
		t.Fatal(err)
	}
	if got.City != "Cuenca" || got.TaxId != "12345678Z" || got.ConsentAt == nil || !got.ConsentAt.Equal(consent) {
		t.Errorf("profile not updated: %+v", got)
	}
	if got.HoldedContactId != "h-1" {
		t.Errorf("holded contact id lost: %q", got.HoldedContactId)
	}
	if !got.CreatedAt.Equal(t1) || !got.UpdatedAt.Equal(t2) {
		t.Errorf("created %v updated %v, want %v and %v", got.CreatedAt, got.UpdatedAt, t1, t2)
	}
	if other, _ := m.GetInvoiceProfile(ctx, "bob"); other == nil || other.Name != "Bob" {
		t.Errorf("bob's profile changed: %+v", other)
	}

	if err = m.DeleteInvoiceProfile(ctx, "ana"); err != nil {
		t.Fatal(err)
	}
	if err = m.DeleteInvoiceProfile(ctx, "ana"); !errors.Is(err, entity.ErrNotFound) {
		t.Errorf("second delete: %v, want ErrNotFound", err)
	}
	if got, err := m.GetInvoiceProfile(ctx, "bob"); err != nil || got == nil {
		t.Errorf("bob's profile deleted with ana's: %v %v", got, err)
	}
	if got, _ := m.GetInvoiceProfile(ctx, "ana"); got != nil {
		t.Errorf("ana's profile survived delete: %+v", got)
	}
}
