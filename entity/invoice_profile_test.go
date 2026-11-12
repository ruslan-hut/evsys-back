package entity

import (
	"strings"
	"testing"
)

func validRequest() InvoiceProfileRequest {
	return InvoiceProfileRequest{
		Enabled: true, Consent: true, Kind: InvoiceKindPerson, Name: "Ana Pérez",
		TaxId: "12345678z", Address: "C. Cuenca 1", City: "El Pedernoso",
		PostalCode: "16690", CountryCode: "es",
	}
}

func TestInvoiceProfileRequestBind(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(r *InvoiceProfileRequest)
		wantErr string // substring, empty = valid
	}{
		{"valid person", func(r *InvoiceProfileRequest) {}, ""},
		{"valid company", func(r *InvoiceProfileRequest) { r.Kind = InvoiceKindCompany; r.TaxId = "A58818501" }, ""},
		{"valid foreign", func(r *InvoiceProfileRequest) { r.CountryCode = "FR"; r.TaxId = "FR12345678901" }, ""},
		{"disabled skips validation", func(r *InvoiceProfileRequest) { *r = InvoiceProfileRequest{} }, ""},
		{"no consent", func(r *InvoiceProfileRequest) { r.Consent = false }, "consent"},
		{"bad kind", func(r *InvoiceProfileRequest) { r.Kind = "other" }, "kind"},
		{"no name", func(r *InvoiceProfileRequest) { r.Name = "  " }, "name"},
		{"no address", func(r *InvoiceProfileRequest) { r.Address = "" }, "address"},
		{"no city", func(r *InvoiceProfileRequest) { r.City = "" }, "city"},
		{"no postal code", func(r *InvoiceProfileRequest) { r.PostalCode = "" }, "postal_code"},
		{"android form: no kind and country", func(r *InvoiceProfileRequest) { r.Kind = ""; r.CountryCode = "" }, ""},
		{"bad country", func(r *InvoiceProfileRequest) { r.CountryCode = "ESP" }, "country_code"},
		{"bad spanish tax id", func(r *InvoiceProfileRequest) { r.TaxId = "12345678A" }, "tax_id"},
		{"no tax id", func(r *InvoiceProfileRequest) { r.TaxId = "" }, "tax_id"},
		{"bad email", func(r *InvoiceProfileRequest) { r.Email = "not-an-email" }, "email"},
		{"several problems at once", func(r *InvoiceProfileRequest) { r.Name = ""; r.City = "" }, "name is required; city is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRequest()
			tt.mutate(&r)
			err := r.Bind(nil)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("error %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestInvoiceProfileRequestBindDefaults(t *testing.T) {
	r := validRequest()
	r.Kind, r.CountryCode = "", ""
	if err := r.Bind(nil); err != nil {
		t.Fatal(err)
	}
	if r.Kind != InvoiceKindPerson || r.CountryCode != "ES" {
		t.Errorf("defaults: kind %q country %q", r.Kind, r.CountryCode)
	}
}

func TestInvoiceProfileRequestBindNormalizes(t *testing.T) {
	r := validRequest()
	r.TaxId = " 12345678-z "
	r.CountryCode = " es "
	if err := r.Bind(nil); err != nil {
		t.Fatal(err)
	}
	if r.TaxId != "12345678Z" || r.CountryCode != "ES" {
		t.Errorf("not normalized: %q %q", r.TaxId, r.CountryCode)
	}
}
