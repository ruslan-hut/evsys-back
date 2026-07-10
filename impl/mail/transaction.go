package mail

import (
	"context"
	"evsys-back/entity"
	"fmt"
	"html"
	"strings"
	"time"
)

// SendTransaction emails a full summary of a single charging session.
func (s *Service) SendTransaction(ctx context.Context, to string, t entity.TransactionMail) error {
	if t.Transaction == nil {
		return fmt.Errorf("transaction is required")
	}
	return s.sender.Send(ctx, to, buildTransactionSubject(t.Transaction), renderTransaction(t))
}

func buildTransactionSubject(tx *entity.Transaction) string {
	return fmt.Sprintf("Charging session #%d — %s", tx.TransactionId, tx.ChargePointId)
}

// transactionDuration mirrors the frontend: prefer the stop timestamp when the
// session is finished, otherwise measure against now.
func transactionDuration(tx *entity.Transaction) time.Duration {
	if tx.TimeStart.IsZero() {
		return 0
	}
	end := tx.TimeStop
	if end.IsZero() || !tx.IsFinished {
		end = time.Now().UTC()
	}
	d := end.Sub(tx.TimeStart)
	if d < 0 {
		return 0
	}
	return d
}

func formatDuration(d time.Duration) string {
	total := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d:%02d", total/3600, (total%3600)/60, total%60)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04:05") + " UTC"
}

// formatEnergy converts watt-hours to kWh.
func formatEnergy(wh int) string {
	return fmt.Sprintf("%.3f kWh", float64(wh)/1000.0)
}

func consumedEnergy(tx *entity.Transaction) int {
	if tx.MeterStop > tx.MeterStart {
		return tx.MeterStop - tx.MeterStart
	}
	return 0
}

// numericCurrencies maps the ISO 4217 numeric codes Redsys returns onto their
// alphabetic form. Unknown codes are shown verbatim.
var numericCurrencies = map[string]string{
	"978": "EUR",
	"840": "USD",
	"826": "GBP",
}

func currencyLabel(code string) string {
	if alpha, ok := numericCurrencies[code]; ok {
		return alpha
	}
	return code
}

// transactionCurrency takes the currency from the first payment order that
// carries one; the transaction itself does not record a currency.
func transactionCurrency(tx *entity.Transaction) string {
	for _, o := range tx.PaymentOrders {
		if o.Currency != "" {
			return currencyLabel(o.Currency)
		}
	}
	return ""
}

// section opens a titled table; rows are appended with row() and the table is
// closed by endSection. Sections with no rows are dropped by the caller.
type section struct {
	title string
	rows  [][2]string
}

func (s *section) row(label, value string) {
	if value == "" {
		return
	}
	s.rows = append(s.rows, [2]string{label, value})
}

func renderTransaction(t entity.TransactionMail) string {
	tx := t.Transaction

	var sections []section

	cp := section{title: "Charge point"}
	cp.row("Charge point", tx.ChargePointId)
	cp.row("Title", t.ChargePointTitle)
	cp.row("Address", t.ChargePointAddress)
	cp.row("Connector", fmt.Sprintf("%d", tx.ConnectorId))
	if tx.EvseId != nil {
		cp.row("EVSE", fmt.Sprintf("%d", *tx.EvseId))
	}
	sections = append(sections, cp)

	sess := section{title: "Session"}
	sess.row("Transaction", fmt.Sprintf("%d", tx.TransactionId))
	sess.row("Session ID", tx.SessionId)
	sess.row("Protocol", tx.ProtocolVersion)
	sess.row("Finished", boolText(tx.IsFinished))
	if tx.ReservationId != nil {
		sess.row("Reservation", fmt.Sprintf("%d", *tx.ReservationId))
	}
	sections = append(sections, sess)

	auth := section{title: "Authentication"}
	auth.row("ID tag", tx.IdTag)
	auth.row("Username", tx.Username)
	auth.row("Note", tx.IdTagNote)
	if tx.UserTag != nil {
		auth.row("Tag source", tx.UserTag.Source)
		auth.row("Tag note", tx.UserTag.Note)
	}
	sections = append(sections, auth)

	tm := section{title: "Time"}
	tm.row("Started", formatTime(tx.TimeStart))
	if tx.IsFinished {
		tm.row("Stopped", formatTime(tx.TimeStop))
	}
	tm.row("Duration", formatDuration(transactionDuration(tx)))
	tm.row("Stop reason", tx.Reason)
	sections = append(sections, tm)

	en := section{title: "Energy"}
	en.row("Meter start", formatEnergy(tx.MeterStart))
	if tx.MeterStop > 0 {
		en.row("Meter stop", formatEnergy(tx.MeterStop))
	}
	en.row("Consumed", formatEnergy(consumedEnergy(tx)))
	sections = append(sections, en)

	currency := transactionCurrency(tx)

	pay := section{title: "Payment"}
	pay.row("Amount", formatAmount(tx.PaymentAmount, currency))
	pay.row("Billed", formatAmount(tx.PaymentBilled, currency))
	if tx.PaymentOrder > 0 {
		pay.row("Order", fmt.Sprintf("%d", tx.PaymentOrder))
	}
	pay.row("Error", tx.PaymentError)
	sections = append(sections, pay)

	if p := tx.Plan; p != nil {
		pl := section{title: "Payment plan"}
		pl.row("Plan", p.PlanId)
		pl.row("Description", p.Description)
		pl.row("Price per kWh", formatAmount(p.PricePerKwh, currency))
		if p.PricePerHour > 0 {
			pl.row("Price per hour", formatAmount(p.PricePerHour, currency))
		}
		sections = append(sections, pl)
	}

	if tf := tx.Tariff; tf != nil {
		tr := section{title: "Tariff"}
		tr.row("Tariff", tf.TariffId)
		tr.row("Description", tf.Description)
		sections = append(sections, tr)
	}

	if pm := tx.PaymentMethod; pm != nil {
		m := section{title: "Payment method"}
		m.row("Description", pm.Description)
		m.row("Card", pm.CardNumber)
		m.row("Brand", pm.CardBrand)
		m.row("Type", pm.CardType)
		m.row("Country", pm.CardCountry)
		m.row("Expires", pm.ExpiryDate)
		sections = append(sections, m)
	}

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html><body style="font-family:Arial,sans-serif;color:#222;">`)
	fmt.Fprintf(&b, `<h2 style="margin-bottom:4px;">Charging session #%d</h2>`, tx.TransactionId)
	fmt.Fprintf(&b, `<p style="color:#666;margin-top:0;">%s &mdash; %s</p>`,
		html.EscapeString(formatTime(tx.TimeStart)),
		html.EscapeString(formatEnergy(consumedEnergy(tx))))

	for _, s := range sections {
		if len(s.rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, `<h3 style="margin-bottom:4px;">%s</h3>`, html.EscapeString(s.title))
		b.WriteString(`<table cellpadding="6" cellspacing="0" border="0" style="border-collapse:collapse;min-width:420px;margin-bottom:12px;">`)
		for _, r := range s.rows {
			fmt.Fprintf(&b,
				`<tr><td style="border-bottom:1px solid #eee;color:#666;">%s</td>`+
					`<td style="border-bottom:1px solid #eee;"><strong>%s</strong></td></tr>`,
				html.EscapeString(r[0]), html.EscapeString(r[1]))
		}
		b.WriteString(`</table>`)
	}

	if len(tx.PaymentOrders) > 0 {
		b.WriteString(`<h3 style="margin-bottom:4px;">Payment orders</h3>`)
		b.WriteString(`<table cellpadding="6" cellspacing="0" border="0" style="border-collapse:collapse;min-width:480px;margin-bottom:12px;">`)
		b.WriteString(`<thead><tr style="background:#f3f3f3;text-align:left;">`)
		b.WriteString(`<th style="border-bottom:1px solid #ccc;">Order</th>`)
		b.WriteString(`<th style="border-bottom:1px solid #ccc;text-align:right;">Amount</th>`)
		b.WriteString(`<th style="border-bottom:1px solid #ccc;">Result</th>`)
		b.WriteString(`<th style="border-bottom:1px solid #ccc;">Opened</th>`)
		b.WriteString(`<th style="border-bottom:1px solid #ccc;">Closed</th>`)
		b.WriteString(`</tr></thead><tbody>`)
		for _, o := range tx.PaymentOrders {
			orderCurrency := currencyLabel(o.Currency)
			amount := formatAmount(o.Amount, orderCurrency)
			if o.RefundAmount > 0 {
				amount = "-" + formatAmount(o.RefundAmount, orderCurrency)
			}
			result := o.Result
			if result == "" {
				result = "—"
			}
			fmt.Fprintf(&b,
				`<tr><td style="border-bottom:1px solid #eee;">#%d</td>`+
					`<td style="border-bottom:1px solid #eee;text-align:right;">%s</td>`+
					`<td style="border-bottom:1px solid #eee;">%s</td>`+
					`<td style="border-bottom:1px solid #eee;">%s</td>`+
					`<td style="border-bottom:1px solid #eee;">%s</td></tr>`,
				o.Order,
				html.EscapeString(amount),
				html.EscapeString(result),
				html.EscapeString(formatTime(o.TimeOpened)),
				html.EscapeString(formatTime(o.TimeClosed)))
		}
		b.WriteString(`</tbody></table>`)
	}

	b.WriteString(`</body></html>`)
	return b.String()
}

func boolText(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}
