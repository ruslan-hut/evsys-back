package mail

import (
	"strings"
	"testing"
	"time"

	"evsys-back/entity"
)

func finishedTransaction() *entity.Transaction {
	start := time.Date(2026, 5, 20, 8, 0, 0, 0, time.UTC)
	return &entity.Transaction{
		TransactionId: 4207,
		ChargePointId: "PE00003",
		ConnectorId:   2,
		IdTag:         "ABC123",
		Username:      "driver@example.com",
		IsFinished:    true,
		MeterStart:    1000,
		MeterStop:     16500,
		TimeStart:     start,
		TimeStop:      start.Add(90 * time.Minute),
		PaymentAmount: 1234,
		PaymentBilled: 1234,
	}
}

func TestBuildTransactionSubject(t *testing.T) {
	got := buildTransactionSubject(finishedTransaction())
	want := "Charging session #4207 — PE00003"
	if got != want {
		t.Errorf("buildTransactionSubject() = %q, want %q", got, want)
	}
}

func TestTransactionDuration(t *testing.T) {
	t.Run("finished uses stop time", func(t *testing.T) {
		if got := transactionDuration(finishedTransaction()); got != 90*time.Minute {
			t.Errorf("duration = %v, want 90m", got)
		}
	})

	t.Run("unfinished measures against now", func(t *testing.T) {
		tx := finishedTransaction()
		tx.IsFinished = false
		tx.TimeStop = time.Time{}
		tx.TimeStart = time.Now().UTC().Add(-30 * time.Minute)
		got := transactionDuration(tx)
		if got < 29*time.Minute || got > 31*time.Minute {
			t.Errorf("duration = %v, want ~30m", got)
		}
	})

	t.Run("zero start yields zero", func(t *testing.T) {
		if got := transactionDuration(&entity.Transaction{}); got != 0 {
			t.Errorf("duration = %v, want 0", got)
		}
	})

	t.Run("stop before start clamps to zero", func(t *testing.T) {
		tx := finishedTransaction()
		tx.TimeStop = tx.TimeStart.Add(-time.Hour)
		if got := transactionDuration(tx); got != 0 {
			t.Errorf("duration = %v, want 0", got)
		}
	})
}

func TestConsumedEnergy(t *testing.T) {
	if got := consumedEnergy(finishedTransaction()); got != 15500 {
		t.Errorf("consumed = %d, want 15500", got)
	}
	// A meter that never advanced must not report negative consumption.
	tx := finishedTransaction()
	tx.MeterStop = 500
	if got := consumedEnergy(tx); got != 0 {
		t.Errorf("consumed = %d, want 0", got)
	}
}

func TestRenderTransaction(t *testing.T) {
	t.Run("includes core session data", func(t *testing.T) {
		body := renderTransaction(entity.TransactionMail{
			Transaction:        finishedTransaction(),
			ChargePointTitle:   "Main street",
			ChargePointAddress: "1 Main st",
		})
		for _, want := range []string{
			"Charging session #4207",
			"PE00003",
			"Main street",
			"1 Main st",
			"ABC123",
			"driver@example.com",
			"15.500 kWh", // consumed
			"01:30:00",   // duration
			"12.34",      // billed amount
			"2026-05-20 08:00:00 UTC",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("body missing %q", want)
			}
		}
	})

	t.Run("omits empty optional sections", func(t *testing.T) {
		body := renderTransaction(entity.TransactionMail{Transaction: finishedTransaction()})
		for _, unwanted := range []string{"Tariff", "Payment method", "Payment orders", "Payment plan"} {
			if strings.Contains(body, unwanted) {
				t.Errorf("body should not contain %q", unwanted)
			}
		}
	})

	t.Run("renders payment orders with refund", func(t *testing.T) {
		tx := finishedTransaction()
		tx.PaymentOrders = []entity.PaymentOrder{
			{Order: 7, Amount: 1234, Currency: "EUR", Result: "0000"},
			{Order: 8, Amount: 1234, RefundAmount: 500, Currency: "EUR", Result: "0900"},
		}
		body := renderTransaction(entity.TransactionMail{Transaction: tx})
		for _, want := range []string{"Payment orders", "#7", "12.34 EUR", "#8", "-5.00 EUR", "0900"} {
			if !strings.Contains(body, want) {
				t.Errorf("body missing %q", want)
			}
		}
	})

	t.Run("maps redsys numeric currency onto the payment section", func(t *testing.T) {
		tx := finishedTransaction()
		tx.PaymentOrders = []entity.PaymentOrder{{Order: 1, Amount: 1234, Currency: "978"}}
		body := renderTransaction(entity.TransactionMail{Transaction: tx})
		if !strings.Contains(body, "12.34 EUR") {
			t.Error("numeric currency 978 was not rendered as EUR")
		}
		if strings.Contains(body, "978") {
			t.Error("raw numeric currency code leaked into the email")
		}
	})

	t.Run("unknown currency code is shown verbatim", func(t *testing.T) {
		if got := currencyLabel("XYZ"); got != "XYZ" {
			t.Errorf("currencyLabel(XYZ) = %q", got)
		}
		if got := currencyLabel(""); got != "" {
			t.Errorf("currencyLabel(empty) = %q", got)
		}
	})

	t.Run("renders the card chrome and preheader", func(t *testing.T) {
		body := renderTransaction(entity.TransactionMail{
			Transaction:      finishedTransaction(),
			ChargePointTitle: "Main Street Station",
		})
		for _, want := range []string{
			`<!DOCTYPE html>`,
			`<title>Charging session #4207</title>`,
			`<!--[if mso]>`,                   // Outlook font fallback
			`width="600"`,                     // fixed card width
			`role="presentation"`,             // tables are layout, not data
			`Session #4207 · 15.500 kWh`,      // preheader
			`PE00003 · Main Street Station`,   // header subline
			`&#10003; Finished &middot; paid`, // status pill
		} {
			if !strings.Contains(body, want) {
				t.Errorf("body missing %q", want)
			}
		}
	})

	t.Run("preheader shows no raw entities", func(t *testing.T) {
		body := renderTransaction(entity.TransactionMail{Transaction: finishedTransaction()})
		preheader := body[strings.Index(body, "opacity:0"):]
		preheader = preheader[:strings.Index(preheader, "</div>")]
		if strings.Contains(preheader, "&amp;#10003;") || strings.Contains(preheader, "#10003") {
			t.Errorf("preheader leaks a checkmark entity: %s", preheader)
		}
	})

	t.Run("status pill reflects session state", func(t *testing.T) {
		cases := []struct {
			name string
			mut  func(*entity.Transaction)
			want string
		}{
			{"paid", func(*entity.Transaction) {}, "Finished &middot; paid"},
			{"charging", func(tx *entity.Transaction) { tx.IsFinished = false }, "Charging"},
			{"failed", func(tx *entity.Transaction) { tx.PaymentError = "0180" }, "Payment failed"},
			{"unpaid", func(tx *entity.Transaction) { tx.PaymentBilled = 0 }, "Finished"},
		}
		for _, c := range cases {
			tx := finishedTransaction()
			c.mut(tx)
			if got := transactionPill(tx).text; !strings.Contains(got, c.want) {
				t.Errorf("%s: pill = %q, want %q", c.name, got, c.want)
			}
		}
	})

	t.Run("meter strip uses a literal arrow, not an escaped entity", func(t *testing.T) {
		body := renderTransaction(entity.TransactionMail{Transaction: finishedTransaction()})
		if strings.Contains(body, "&amp;rarr;") {
			t.Error("arrow entity was double-escaped")
		}
		if !strings.Contains(body, "1.000 → 16.500 kWh") {
			t.Error("meter range missing")
		}
	})

	t.Run("running session shows meter start instead of a range", func(t *testing.T) {
		tx := finishedTransaction()
		tx.IsFinished = false
		tx.MeterStop = 0
		body := renderTransaction(entity.TransactionMail{Transaction: tx})
		if strings.Contains(body, "→") {
			t.Error("running session must not render a meter range")
		}
		if !strings.Contains(body, "Meter start") {
			t.Error("running session should show the start reading")
		}
	})

	t.Run("running session shows unknown energy and amount as a dash", func(t *testing.T) {
		tx := finishedTransaction()
		tx.IsFinished = false
		tx.MeterStop = 0
		tx.PaymentAmount = 0
		tx.PaymentBilled = 0
		body := renderTransaction(entity.TransactionMail{Transaction: tx})

		if strings.Contains(body, "0.000") {
			t.Error("unknown energy rendered as a false zero")
		}
		if strings.Contains(body, "0.00 ") || strings.Contains(body, ">0.00<") {
			t.Error("unknown amount rendered as a false zero")
		}
		if !strings.Contains(body, "—") {
			t.Error("expected an em dash for the unknown metrics")
		}
		// Empty payment rows must not appear either.
		if strings.Contains(body, "Billed") {
			t.Error("unsettled session should not show a Billed row")
		}
	})

	t.Run("a finished session that consumed nothing still shows zero", func(t *testing.T) {
		tx := finishedTransaction()
		tx.MeterStop = tx.MeterStart
		body := renderTransaction(entity.TransactionMail{Transaction: tx})
		if !strings.Contains(body, "0.000") {
			t.Error("a finished zero-consumption session should report 0.000, not a dash")
		}
	})

	t.Run("amount falls back to the computed amount before settlement", func(t *testing.T) {
		tx := finishedTransaction()
		tx.PaymentBilled = 0
		tx.PaymentAmount = 999
		if got := billedAmount(tx); got != 999 {
			t.Errorf("billedAmount = %d, want 999", got)
		}
		body := renderTransaction(entity.TransactionMail{Transaction: tx})
		if !strings.Contains(body, "9.99") {
			t.Error("computed amount missing from the KPI row")
		}
	})

	t.Run("footer does not claim an unsettled order was closed", func(t *testing.T) {
		tx := finishedTransaction()
		tx.PaymentOrders = []entity.PaymentOrder{{Order: 3000, Amount: 400, Currency: "978", Result: "0180"}}
		body := renderTransaction(entity.TransactionMail{Transaction: tx})
		if strings.Contains(body, "closed —") {
			t.Error("footer claims an unclosed order was closed")
		}
		if !strings.Contains(body, "Order #3000 (result 0180)") {
			t.Error("footer should name the order and result without a close time")
		}
	})

	t.Run("footer reports the close time of a settled order", func(t *testing.T) {
		tx := finishedTransaction()
		closed := time.Date(2026, 5, 20, 9, 31, 0, 0, time.UTC)
		tx.PaymentOrders = []entity.PaymentOrder{
			{Order: 2863, Amount: 637, Currency: "978", Result: "0000", TimeClosed: closed},
		}
		body := renderTransaction(entity.TransactionMail{Transaction: tx})
		if !strings.Contains(body, "Order #2863 closed 2026-05-20 09:31:00 UTC (result 0000)") {
			t.Error("footer missing the settled order summary")
		}
	})

	t.Run("headline energy is two decimals, detail rows keep three", func(t *testing.T) {
		body := renderTransaction(entity.TransactionMail{Transaction: finishedTransaction()})
		if !strings.Contains(body, `">15.50 <span`) {
			t.Error("KPI energy should render as 15.50 followed by its unit span")
		}
		if !strings.Contains(body, "15.500 kWh") {
			t.Error("full precision should survive outside the KPI")
		}
	})

	t.Run("escapes html in user controlled fields", func(t *testing.T) {
		tx := finishedTransaction()
		tx.IdTagNote = `<script>alert(1)</script>`
		body := renderTransaction(entity.TransactionMail{Transaction: tx})
		if strings.Contains(body, "<script>") {
			t.Error("note was not escaped")
		}
		if !strings.Contains(body, "&lt;script&gt;") {
			t.Error("escaped note missing")
		}
	})

	t.Run("unfinished session omits stop time", func(t *testing.T) {
		tx := finishedTransaction()
		tx.IsFinished = false
		tx.TimeStop = time.Time{}
		body := renderTransaction(entity.TransactionMail{Transaction: tx})
		if strings.Contains(body, "Stopped") {
			t.Error("unfinished session should not render a stop time")
		}
	})
}
