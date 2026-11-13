package holded

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(Config{ApiKey: "secret-key", ApiUrl: srv.URL + "/api/v2/", Timeout: 2 * time.Second},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func asError(t *testing.T, err error) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("want *Error, got %T: %v", err, err)
	}
	return e
}

func TestCreateContact(t *testing.T) {
	var got map[string]any
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/contacts" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"c-1"}`))
	})

	id, err := c.CreateContact(context.Background(), Contact{
		Name: "Ana Pérez", VatId: "12345678Z", IsPerson: true, Email: "ana@example.com",
		Address: Address{Address: "C. Mayor 1", City: "Cuenca", PostalCode: "16001", CountryCode: "ES"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "c-1" {
		t.Errorf("id = %q", id)
	}
	if got["name"] != "Ana Pérez" || got["vat_number"] != "12345678Z" || got["is_person"] != true {
		t.Errorf("body = %v", got)
	}
	addr, _ := got["bill_address"].(map[string]any)
	if addr["city"] != "Cuenca" || addr["postal_code"] != "16001" || addr["country_code"] != "ES" {
		t.Errorf("bill_address = %v", addr)
	}
	if _, ok := got["custom_id"]; ok {
		t.Errorf("empty custom_id must be omitted: %v", got)
	}
}

func TestCreateInvoice(t *testing.T) {
	var got struct {
		ContactId    string         `json:"contact_id"`
		NumberLineId string         `json:"number_line_id"`
		CustomFields map[string]any `json:"custom_fields"`
		Items        []Item         `json:"items"`
	}
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/invoices" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"inv-9"}`))
	})

	id, err := c.CreateInvoice(context.Background(), Invoice{
		ContactId:    "c-1",
		NumberLineId: "line-1",
		CustomFields: map[string]any{"transaction_id": 5337},
		Items:        []Item{{Name: "Carga EV", Units: 32.676, Price: Euros(1209), Taxes: []string{"s_iva_21"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "inv-9" {
		t.Errorf("id = %q", id)
	}
	if got.ContactId != "c-1" || got.NumberLineId != "line-1" || got.CustomFields["transaction_id"] != float64(5337) {
		t.Errorf("body = %+v", got)
	}
	if len(got.Items) != 1 || got.Items[0].Price != 12.09 || got.Items[0].Taxes[0] != "s_iva_21" {
		t.Errorf("items = %+v", got.Items)
	}
}

func TestCreateInvoiceRejectsIncompleteRequest(t *testing.T) {
	c := testClient(t, func(http.ResponseWriter, *http.Request) { t.Error("request must not be sent") })
	if _, err := c.CreateInvoice(context.Background(), Invoice{Items: []Item{{Name: "x"}}}); err == nil {
		t.Error("invoice without contact accepted")
	}
	if _, err := c.CreateInvoice(context.Background(), Invoice{ContactId: "c-1"}); err == nil {
		t.Error("invoice without items accepted")
	}
}

func TestInvoiceSteps(t *testing.T) {
	var calls []string
	var payment Payment
	var mail Mail
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/api/v2/invoices/inv 1/payments":
			_ = json.NewDecoder(r.Body).Decode(&payment)
		case "/api/v2/invoices/inv 1/send":
			_ = json.NewDecoder(r.Body).Decode(&mail)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":1}`))
	})
	ctx := context.Background()

	if err := c.ApproveInvoice(ctx, "inv 1"); err != nil {
		t.Fatal(err)
	}
	if err := c.AddPayment(ctx, "inv 1", Payment{Amount: 12.09, TreasuryId: "t-1"}); err != nil {
		t.Fatal(err)
	}
	if err := c.SendInvoice(ctx, "inv 1", Mail{Emails: []string{"ana@example.com"}}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"POST /api/v2/invoices/inv 1/approve",
		"POST /api/v2/invoices/inv 1/payments",
		"POST /api/v2/invoices/inv 1/send",
	}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v", calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call %d = %q, want %q", i, calls[i], want[i])
		}
	}
	if payment.Amount != 12.09 || payment.TreasuryId != "t-1" {
		t.Errorf("payment = %+v", payment)
	}
	if len(mail.Emails) != 1 || mail.Emails[0] != "ana@example.com" {
		t.Errorf("mail = %+v", mail)
	}
	if err := c.SendInvoice(ctx, "inv 1", Mail{}); err == nil {
		t.Error("send without recipients accepted")
	}
}

func TestErrorClasses(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		retryAfter string
		retryable  bool
		ambiguous  bool
		auth       bool
		wait       time.Duration
	}{
		{name: "validation", status: 422},
		{name: "bad request", status: 400},
		{name: "not found", status: 404},
		{name: "unauthorized", status: 401, auth: true},
		{name: "forbidden", status: 403, auth: true},
		{name: "rate limit seconds", status: 429, retryAfter: "7", retryable: true, wait: 7 * time.Second},
		{name: "rate limit no header", status: 429, retryable: true},
		{name: "server error", status: 500, retryable: true, ambiguous: true},
		{name: "bad gateway", status: 502, retryable: true, ambiguous: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"error":"nope"}`))
			})
			_, err := c.CreateInvoice(context.Background(), Invoice{ContactId: "c", Items: []Item{{Name: "x"}}})
			e := asError(t, err)
			if e.Status != tc.status {
				t.Errorf("status = %d", e.Status)
			}
			if e.Retryable() != tc.retryable || e.Ambiguous() != tc.ambiguous || e.Unauthorized() != tc.auth {
				t.Errorf("retryable=%v ambiguous=%v unauthorized=%v", e.Retryable(), e.Ambiguous(), e.Unauthorized())
			}
			if e.RetryAfter != tc.wait {
				t.Errorf("retry after = %v, want %v", e.RetryAfter, tc.wait)
			}
			if e.Body != `{"error":"nope"}` {
				t.Errorf("body = %q", e.Body)
			}
		})
	}
}

func TestErrorDoesNotLeakKey(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	_, err := c.CreateContact(context.Background(), Contact{Name: "x"})
	if err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Errorf("error = %v", err)
	}
}

func TestTimeoutIsAmbiguousAndRetryable(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	c := New(Config{ApiKey: "k", ApiUrl: srv.URL, Timeout: 50 * time.Millisecond},
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := c.CreateInvoice(context.Background(), Invoice{ContactId: "c", Items: []Item{{Name: "x"}}})
	e := asError(t, err)
	if e.Status != 0 || !e.Retryable() || !e.Ambiguous() {
		t.Errorf("status=%d retryable=%v ambiguous=%v", e.Status, e.Retryable(), e.Ambiguous())
	}
}

func TestCanceledContext(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.CreateContact(ctx, Contact{Name: "x"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v", err)
	}
}

func TestResponseWithoutIdIsAmbiguous(t *testing.T) {
	for name, body := range map[string]string{"empty id": `{}`, "not json": `<html>`} {
		t.Run(name, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(body))
			})
			_, err := c.CreateInvoice(context.Background(), Invoice{ContactId: "c", Items: []Item{{Name: "x"}}})
			e := asError(t, err)
			if !e.Ambiguous() {
				t.Errorf("a created document with an unreadable answer must be ambiguous: %v", e)
			}
		})
	}
}

func TestRetryAfterDate(t *testing.T) {
	if d := parseRetryAfter(time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)); d <= 20*time.Second || d > 31*time.Second {
		t.Errorf("date form = %v", d)
	}
	if d := parseRetryAfter("soon"); d != 0 {
		t.Errorf("garbage = %v", d)
	}
}

func TestEuros(t *testing.T) {
	for cents, want := range map[int]float64{0: 0, 1: 0.01, 1209: 12.09, 487: 4.87, 100000: 1000} {
		if got := Euros(cents); got != want {
			t.Errorf("Euros(%d) = %v, want %v", cents, got, want)
		}
	}
}
