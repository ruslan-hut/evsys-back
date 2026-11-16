// Package holded is a thin client of the Holded invoicing API (v2).
//
// It only moves data: the rules (when an invoice is issued, which tax, whether
// Holded or our mail sends it) belong to the caller and are still open in the
// invoicing task series. Request and response shapes follow the public
// documentation read on 2026-11-05; nothing here has been run against a live
// Holded account yet (no API key), so field names and the response of calls
// other than the creating ones are a hypothesis until the pilot.
package holded

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"evsys-back/internal/lib/sl"
)

// Config holds the Holded connection settings.
type Config struct {
	ApiKey  string
	ApiUrl  string
	Timeout time.Duration
}

// Client calls the Holded HTTP API. It is safe for concurrent use.
type Client struct {
	cfg  Config
	http *http.Client
	log  *slog.Logger
}

func New(cfg Config, log *slog.Logger) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	cfg.ApiUrl = strings.TrimRight(cfg.ApiUrl, "/")
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
		log:  log.With(sl.Module("holded.client")),
	}
}

// Address is a postal address of a contact.
type Address struct {
	Address     string `json:"address,omitempty"`
	City        string `json:"city,omitempty"`
	PostalCode  string `json:"postal_code,omitempty"`
	Province    string `json:"province,omitempty"`
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
}

// Contact is a customer. Name is the only field Holded requires.
type Contact struct {
	Name     string  `json:"name"`
	VatId    string  `json:"vat_number,omitempty"`
	IsPerson bool    `json:"is_person"`
	Email    string  `json:"email,omitempty"`
	CustomId string  `json:"custom_id,omitempty"`
	Address  Address `json:"bill_address"`
}

// Item is one invoice line. Price is in euros, as Holded takes it; whether it
// includes the tax is not settled (Holded docs do not say), so the caller
// decides and checks the total against the amount charged.
type Item struct {
	Name  string   `json:"name"`
	Units float64  `json:"units"`
	Price float64  `json:"price"`
	Taxes []string `json:"taxes,omitempty"`
}

// Invoice is a sales invoice. NumberLineId selects the numbering series;
// setting the number by hand breaks the series, so it is not offered.
// CustomFields lets us store our transaction id on the document: Holded has no
// idempotency key and no filter by custom field, so a lost response to a create
// is recovered by listing and comparing on our side.
type Invoice struct {
	ContactId    string         `json:"contact_id"`
	Items        []Item         `json:"items"`
	Date         string         `json:"date,omitempty"`
	NumberLineId string         `json:"number_line_id,omitempty"`
	CustomFields map[string]any `json:"custom_fields,omitempty"`
}

// Payment registers a payment against an invoice.
type Payment struct {
	Amount     float64 `json:"amount"`
	TreasuryId string  `json:"treasury_id,omitempty"`
	Date       string  `json:"date,omitempty"`
}

// Mail asks Holded to email the invoice.
type Mail struct {
	Emails  []string `json:"emails"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message,omitempty"`
}

// Euros converts an amount in cents, as evsys stores money, to euros.
func Euros(cents int) float64 {
	return float64(cents) / 100
}

// CreateContact creates a contact and returns its Holded id. Holded does not
// guard against duplicates: keep the id and do not create a contact twice.
func (c *Client) CreateContact(ctx context.Context, contact Contact) (string, error) {
	return c.create(ctx, "create contact", "/contacts", contact)
}

// CreateInvoice creates an invoice and returns its Holded id.
//
// A failure may be ambiguous (see Error.Ambiguous): the request can have
// reached Holded and created the document. Do not repeat the call blindly.
func (c *Client) CreateInvoice(ctx context.Context, invoice Invoice) (string, error) {
	if invoice.ContactId == "" {
		return "", errors.New("holded: invoice without contact")
	}
	if len(invoice.Items) == 0 {
		return "", errors.New("holded: invoice without items")
	}
	return c.create(ctx, "create invoice", "/invoices", invoice)
}

// ApproveInvoice turns a draft into an issued invoice.
func (c *Client) ApproveInvoice(ctx context.Context, id string) error {
	return c.do(ctx, "approve invoice", http.MethodPost, "/invoices/"+url.PathEscape(id)+"/approve", nil, nil)
}

// AddPayment registers a payment on an invoice.
func (c *Client) AddPayment(ctx context.Context, id string, payment Payment) error {
	return c.do(ctx, "add payment", http.MethodPost, "/invoices/"+url.PathEscape(id)+"/payments", payment, nil)
}

// SendInvoice asks Holded to email the invoice to the given addresses.
func (c *Client) SendInvoice(ctx context.Context, id string, mail Mail) error {
	if len(mail.Emails) == 0 {
		return errors.New("holded: no recipients")
	}
	return c.do(ctx, "send invoice", http.MethodPost, "/invoices/"+url.PathEscape(id)+"/send", mail, nil)
}

func (c *Client) create(ctx context.Context, op, path string, payload any) (string, error) {
	var created struct {
		Id string `json:"id"`
	}
	if err := c.do(ctx, op, http.MethodPost, path, payload, &created); err != nil {
		return "", err
	}
	if created.Id == "" {
		// the document may exist all the same: treat it as ambiguous
		return "", &Error{Op: op, Status: http.StatusCreated, Err: errors.New("response without id")}
	}
	return created.Id, nil
}

func (c *Client) do(ctx context.Context, op, method, path string, payload, result any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("holded %s: marshal: %w", op, err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.ApiUrl+path, body)
	if err != nil {
		return fmt.Errorf("holded %s: build request: %w", op, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.ApiKey)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// the request may have been sent and processed: the answer is lost
		return &Error{Op: op, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		e := &Error{Op: op, Status: resp.StatusCode, Body: truncate(string(data), 500)}
		if resp.StatusCode == http.StatusTooManyRequests {
			e.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		}
		c.log.Warn("holded request failed", slog.String("op", op), slog.Int("status", resp.StatusCode))
		return e
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return &Error{Op: op, Status: resp.StatusCode, Err: fmt.Errorf("decode response: %w", err)}
		}
	}
	c.log.Debug("holded request", slog.String("op", op), slog.Int("status", resp.StatusCode))
	return nil
}

// Error is a failed Holded call. Status is zero when no answer came.
type Error struct {
	Op         string
	Status     int
	Body       string
	RetryAfter time.Duration
	Err        error
}

func (e *Error) Error() string {
	switch {
	case e.Err != nil && e.Status != 0:
		return fmt.Sprintf("holded %s: status %d: %v", e.Op, e.Status, e.Err)
	case e.Err != nil:
		return fmt.Sprintf("holded %s: %v", e.Op, e.Err)
	default:
		return fmt.Sprintf("holded %s: status %d: %s", e.Op, e.Status, e.Body)
	}
}

func (e *Error) Unwrap() error { return e.Err }

// Retryable reports whether the same call may succeed later: no answer, rate
// limit, or a server error. Other statuses are permanent until the data or the
// configuration changes.
func (e *Error) Retryable() bool {
	return e.Status == 0 || e.Status == http.StatusTooManyRequests || e.Status >= 500
}

// Ambiguous reports whether the request may have been processed although the
// call failed: no answer, a server error, or a response that could not be
// read. A create that failed this way can have left a document behind.
func (e *Error) Ambiguous() bool {
	return e.Status == 0 || e.Status >= 500 || e.Err != nil
}

// Unauthorized reports a rejected key or a missing permission: an operator
// problem, not a property of the invoice.
func (e *Error) Unauthorized() bool {
	return e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden
}

// Wait is how long Holded asked to wait before the next call (Retry-After of a
// 429), zero if it did not say.
func (e *Error) Wait() time.Duration { return e.RetryAfter }

func parseRetryAfter(v string) time.Duration {
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
