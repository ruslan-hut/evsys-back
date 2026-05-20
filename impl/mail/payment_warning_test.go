package mail

import (
	"strings"
	"testing"
	"time"

	"evsys-back/entity"
)

func TestBuildWarningSubject(t *testing.T) {
	tests := []struct {
		name string
		w    entity.PaymentWarning
		want string
	}{
		{
			name: "retry attempt",
			w:    entity.PaymentWarning{TransactionId: 4207, ChargePointId: "PE00003", Attempt: 2, MaxAttempts: 4},
			want: "Payment failed — attempt 2/4 — PE00003 tx 4207",
		},
		{
			name: "exhausted",
			w:    entity.PaymentWarning{TransactionId: 4207, ChargePointId: "PE00003", MaxAttempts: 4, Exhausted: true},
			want: "Payment failed — retries exhausted — PE00003 tx 4207",
		},
		{
			name: "no charge point",
			w:    entity.PaymentWarning{TransactionId: 99},
			want: "Payment failed — — tx 99",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildWarningSubject(tt.w); got != tt.want {
				t.Errorf("buildWarningSubject() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderPaymentWarning(t *testing.T) {
	now := time.Date(2026, 5, 20, 10, 2, 9, 0, time.UTC)

	t.Run("retry state and amount", func(t *testing.T) {
		body := renderPaymentWarning(entity.PaymentWarning{
			TransactionId: 4207,
			OrderNumber:   2902,
			ChargePointId: "PE00003",
			CardHolder:    "user_ed7ee",
			Amount:        1555,
			Currency:      "978",
			Result:        "SIS0334",
			Attempt:       2,
			MaxAttempts:   4,
			OccurredAt:    now,
		})
		for _, want := range []string{
			"15.55 978",
			"SIS0334",
			"Retry attempt 2 of 4",
			"2026-05-20 10:02:09 UTC",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("body missing %q\n%s", want, body)
			}
		}
	})

	t.Run("exhausted wording", func(t *testing.T) {
		body := renderPaymentWarning(entity.PaymentWarning{
			TransactionId: 4207, MaxAttempts: 4, Exhausted: true, OccurredAt: now,
		})
		if !strings.Contains(body, "All 4 retry attempts exhausted") {
			t.Errorf("body missing exhausted wording\n%s", body)
		}
		if strings.Contains(body, "Retry attempt") {
			t.Errorf("exhausted body should not mention a pending retry\n%s", body)
		}
	})

	t.Run("escapes html in result", func(t *testing.T) {
		body := renderPaymentWarning(entity.PaymentWarning{
			TransactionId: 1, Result: "<script>x</script>", OccurredAt: now,
		})
		if strings.Contains(body, "<script>x</script>") {
			t.Errorf("result was not html-escaped\n%s", body)
		}
		if !strings.Contains(body, "&lt;script&gt;") {
			t.Errorf("expected escaped result\n%s", body)
		}
	})
}
