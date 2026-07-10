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
