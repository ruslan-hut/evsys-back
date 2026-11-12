package entity

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"evsys-back/internal/lib/validate"
)

// Invoice profile kinds.
const (
	InvoiceKindPerson  = "person"
	InvoiceKindCompany = "company"
)

// InvoiceProfile holds what a user entered to receive invoices for charging
// sessions. It is stored apart from User (collection invoice_profiles, one
// document per user) and is only ever returned to its owner: never in user
// lists, never to other users.
//
// ConsentAt and HoldedContactId are set by the server. HoldedContactId is the
// customer's contact in Holded; it is never exposed over the API.
type InvoiceProfile struct {
	UserId          string     `json:"-" bson:"user_id"`
	Enabled         bool       `json:"enabled" bson:"enabled"`
	Kind            string     `json:"kind" bson:"kind"`
	Name            string     `json:"name" bson:"name"`
	TaxId           string     `json:"tax_id" bson:"tax_id"`
	Email           string     `json:"email,omitempty" bson:"email,omitempty"`
	Address         string     `json:"address" bson:"address"`
	City            string     `json:"city" bson:"city"`
	PostalCode      string     `json:"postal_code" bson:"postal_code"`
	Province        string     `json:"province,omitempty" bson:"province,omitempty"`
	CountryCode     string     `json:"country_code" bson:"country_code"`
	ConsentAt       *time.Time `json:"consent_at,omitempty" bson:"consent_at,omitempty"`
	HoldedContactId string     `json:"-" bson:"holded_contact_id,omitempty"`
	CreatedAt       time.Time  `json:"-" bson:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" bson:"updated_at"`
}

// InvoiceProfileRequest is what the client sends. Consent is the user's
// explicit agreement to the processing of the tax data; the server turns it
// into ConsentAt.
type InvoiceProfileRequest struct {
	Enabled     bool   `json:"enabled"`
	Consent     bool   `json:"consent"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	TaxId       string `json:"tax_id"`
	Email       string `json:"email"`
	Address     string `json:"address"`
	City        string `json:"city"`
	PostalCode  string `json:"postal_code"`
	Province    string `json:"province"`
	CountryCode string `json:"country_code"`
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Bind normalizes the request and, when invoices are being enabled, checks the
// required fields. A request that only turns invoices off is not validated:
// the user must always be able to opt out.
func (r *InvoiceProfileRequest) Bind(_ *http.Request) error {
	r.Kind = strings.TrimSpace(r.Kind)
	r.Name = strings.TrimSpace(r.Name)
	r.Email = strings.TrimSpace(r.Email)
	r.Address = strings.TrimSpace(r.Address)
	r.City = strings.TrimSpace(r.City)
	r.PostalCode = strings.TrimSpace(r.PostalCode)
	r.Province = strings.TrimSpace(r.Province)
	r.CountryCode = strings.ToUpper(strings.TrimSpace(r.CountryCode))
	// the Android form has no kind and no country: individuals in Spain
	// (and no postal code yet: the app has to add the field)
	if r.Kind == "" {
		r.Kind = InvoiceKindPerson
	}
	if r.CountryCode == "" {
		r.CountryCode = "ES"
	}
	r.TaxId = validate.NormalizeTaxId(r.TaxId)

	if !r.Enabled {
		return nil
	}

	var problems []string
	if r.Kind != InvoiceKindPerson && r.Kind != InvoiceKindCompany {
		problems = append(problems, "kind must be person or company")
	}
	if r.Name == "" {
		problems = append(problems, "name is required")
	}
	if r.Address == "" {
		problems = append(problems, "address is required")
	}
	if r.City == "" {
		problems = append(problems, "city is required")
	}
	if r.PostalCode == "" {
		problems = append(problems, "postal_code is required")
	}
	if len(r.CountryCode) != 2 {
		problems = append(problems, "country_code must be a two-letter ISO 3166-1 code")
	} else if err := validate.TaxId(r.CountryCode, r.TaxId); err != nil {
		problems = append(problems, "tax_id: "+err.Error())
	}
	if r.Email != "" && !emailRe.MatchString(r.Email) {
		problems = append(problems, "email is not valid")
	}
	if !r.Consent {
		problems = append(problems, "consent is required to receive invoices")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
