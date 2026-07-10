package mail

import (
	"context"
	"evsys-back/entity"
	"fmt"
	"html"
	"strings"
	"time"
)

// Palette and metrics of the transaction email, kept in one place because every
// rule has to be inlined on the element that uses it: mail clients drop <style>.
const (
	colText     = "#1a1a1a"
	colMuted    = "#6b6f76"
	colLabel    = "#9a9ea6"
	colBorder   = "#e3e5e8"
	colRule     = "#eeeff1"
	colPage     = "#f2f3f5"
	colStrip    = "#f7f8f9"
	colFooterBg = "#fafbfc"

	fontStack = "Arial,Helvetica,sans-serif"
	monoStack = "Consolas,Menlo,monospace"
)

// SendTransaction emails a full summary of a single charging session.
func (s *Service) SendTransaction(ctx context.Context, to string, t entity.TransactionMail) error {
	if t.Transaction == nil {
		return fmt.Errorf("transaction is required")
	}
	return s.sender.Send(ctx, to, buildTransactionSubject(t.Transaction), renderTransaction(t))
}

// RenderTransaction returns the standalone HTML document for a charging
// session. It is the same markup the email carries, exported so the printable
// receipt cannot drift from it. Callers need no mail provider.
func RenderTransaction(t entity.TransactionMail) string {
	if t.Transaction == nil {
		return ""
	}
	return renderTransaction(t)
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

// formatShortTime is the header subline form: no seconds.
func formatShortTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04") + " UTC"
}

// formatEnergy converts watt-hours to kWh.
func formatEnergy(wh int) string {
	return fmt.Sprintf("%.3f kWh", float64(wh)/1000.0)
}

// formatEnergyValue is formatEnergy without the unit, for the meter strip where
// the unit is written once at the end of the range.
func formatEnergyValue(wh int) string {
	return fmt.Sprintf("%.3f", float64(wh)/1000.0)
}

// formatEnergyHeadline trades a digit of precision for legibility in the KPI
// cell, which renders its unit in a separate span.
func formatEnergyHeadline(wh int) string {
	return fmt.Sprintf("%.2f", float64(wh)/1000.0)
}

func consumedEnergy(tx *entity.Transaction) int {
	if tx.MeterStop > tx.MeterStart {
		return tx.MeterStop - tx.MeterStart
	}
	return 0
}

// energyKnown reports whether consumption can be derived from the transaction.
// A running session has no meter_stop yet, so a rendered "0.000 kWh" would
// claim the driver drew nothing rather than that we do not know.
func energyKnown(tx *entity.Transaction) bool {
	return tx.IsFinished || consumedEnergy(tx) > 0
}

// billedAmount is what the session actually cost: the billed figure once
// settled, otherwise the amount computed at stop. Zero means "nothing to show".
func billedAmount(tx *entity.Transaction) int {
	if tx.PaymentBilled > 0 {
		return tx.PaymentBilled
	}
	return tx.PaymentAmount
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

func boolText(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}

// --- status pill ---

type pill struct {
	text string
	fg   string
	bg   string
}

func transactionPill(tx *entity.Transaction) pill {
	switch {
	case tx.PaymentError != "":
		return pill{"Payment failed", "#b00020", "#fdecef"}
	case !tx.IsFinished:
		return pill{"Charging", "#0b5cad", "#e3f0fb"}
	case tx.PaymentBilled > 0:
		return pill{"&#10003; Finished &middot; paid", "#0f6e56", "#e1f5ee"}
	default:
		return pill{"Finished", colMuted, colRule}
	}
}

// --- section model ---

type row struct {
	label string
	value string
	mono  bool
}

type section struct {
	title string
	rows  []row
}

func (s *section) add(label, value string) {
	if value == "" {
		return
	}
	s.rows = append(s.rows, row{label: label, value: value})
}

func (s *section) addMono(label, value string) {
	if value == "" {
		return
	}
	s.rows = append(s.rows, row{label: label, value: value, mono: true})
}

// renderSection writes one uppercase caption plus its label/value rows. The last
// row drops its rule so sections do not end on a dangling line.
func renderSection(b *strings.Builder, s section, first bool) {
	padding := "padding:14px 0 4px 0;"
	if first {
		padding = "padding-bottom:4px;"
	}
	fmt.Fprintf(b,
		`<div style="font-size:11px;font-weight:bold;text-transform:uppercase;letter-spacing:0.4px;color:%s;%s">%s</div>`,
		colLabel, padding, html.EscapeString(s.title))

	fmt.Fprintf(b, `<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="font-size:13px;">`)
	for i, r := range s.rows {
		rule := fmt.Sprintf("border-bottom:1px solid %s;", colRule)
		if i == len(s.rows)-1 {
			rule = ""
		}
		valueFont := ""
		if r.mono {
			valueFont = "font-family:" + monoStack + ";"
		}
		fmt.Fprintf(b,
			`<tr><td style="color:%s;padding:5px 0;%s">%s</td>`+
				`<td align="right" style="color:%s;font-weight:bold;%spadding:5px 0;%s">%s</td></tr>`,
			colMuted, rule, html.EscapeString(r.label),
			colText, valueFont, rule, html.EscapeString(r.value))
	}
	b.WriteString(`</table>`)
}

func renderColumn(b *strings.Builder, sections []section) {
	first := true
	for _, s := range sections {
		if len(s.rows) == 0 {
			continue
		}
		renderSection(b, s, first)
		first = false
	}
}

// kpi renders one cell of the three-up metric row.
func kpi(b *strings.Builder, label, value, unit string, rightRule bool) {
	rule := ""
	if rightRule {
		rule = fmt.Sprintf("border-right:1px solid %s;", colBorder)
	}
	unitSpan := ""
	if unit != "" {
		unitSpan = fmt.Sprintf(
			` <span style="font-size:13px;font-weight:normal;color:%s;">%s</span>`,
			colMuted, html.EscapeString(unit))
	}
	fmt.Fprintf(b,
		`<td width="33.33%%" valign="top" style="padding:16px 24px;border-bottom:1px solid %s;%s">`+
			`<div style="font-size:12px;color:%s;">%s</div>`+
			`<div style="font-size:22px;font-weight:bold;color:%s;line-height:1.2;margin-top:2px;">%s%s</div></td>`,
		colBorder, rule, colMuted, html.EscapeString(label), colText, html.EscapeString(value), unitSpan)
}

// stripItem is one "Label value" pair of the full-width session strip.
func stripItem(label, value string) string {
	return fmt.Sprintf(`<span style="color:%s;font-weight:bold;">%s</span> %s`,
		colText, html.EscapeString(label), html.EscapeString(value))
}

func renderTransaction(t entity.TransactionMail) string {
	tx := t.Transaction
	currency := transactionCurrency(tx)
	consumed := consumedEnergy(tx)
	status := transactionPill(tx)

	// --- left column ---
	cp := section{title: "Charge point"}
	cp.add("Charge point", tx.ChargePointId)
	cp.add("Connector", fmt.Sprintf("%d", tx.ConnectorId))
	if tx.EvseId != nil {
		cp.add("EVSE", fmt.Sprintf("%d", *tx.EvseId))
	}
	cp.add("Address", t.ChargePointAddress)

	sess := section{title: "Session"}
	sess.add("Transaction", fmt.Sprintf("%d", tx.TransactionId))
	sess.addMono("Session ID", tx.SessionId)
	sess.add("Protocol", tx.ProtocolVersion)
	sess.add("Finished", boolText(tx.IsFinished))
	if tx.ReservationId != nil {
		sess.add("Reservation", fmt.Sprintf("%d", *tx.ReservationId))
	}

	tariff := section{title: "Tariff"}
	if p := tx.Plan; p != nil {
		tariff.add("Plan", p.PlanId)
		tariff.add("Description", p.Description)
		if p.PricePerKwh > 0 {
			tariff.add("Price", formatAmount(p.PricePerKwh, currency)+"/kWh")
		}
		if p.PricePerHour > 0 {
			tariff.add("Price", formatAmount(p.PricePerHour, currency)+"/h")
		}
	}
	if tf := tx.Tariff; tf != nil {
		tariff.add("Tariff", tf.TariffId)
		tariff.add("Tariff description", tf.Description)
	}

	// --- right column ---
	user := section{title: "User"}
	user.add("Username", tx.Username)
	user.addMono("ID tag", tx.IdTag)
	user.add("Note", tx.IdTagNote)
	if ut := tx.UserTag; ut != nil {
		user.add("Tag source", ut.Source)
		user.add("Tag note", ut.Note)
	}

	pay := section{title: "Payment"}
	if tx.PaymentOrder > 0 {
		pay.add("Order", fmt.Sprintf("#%d", tx.PaymentOrder))
	}
	if tx.PaymentAmount > 0 {
		pay.add("Amount", formatAmount(tx.PaymentAmount, currency))
	}
	if tx.PaymentBilled > 0 {
		pay.add("Billed", formatAmount(tx.PaymentBilled, currency))
	}
	pay.add("Stop reason", tx.Reason)
	pay.add("Error", tx.PaymentError)

	method := section{title: "Payment method"}
	if pm := tx.PaymentMethod; pm != nil {
		method.add("Description", pm.Description)
		method.addMono("Card", pm.CardNumber)
		method.add("Brand", pm.CardBrand)
		method.add("Type", pm.CardType)
		method.add("Country", pm.CardCountry)
		method.add("Expires", pm.ExpiryDate)
	}

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>` + "\n")
	b.WriteString(`<html lang="en" xmlns="http://www.w3.org/1999/xhtml">` + "\n<head>\n")
	b.WriteString(`<meta charset="utf-8">` + "\n")
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1.0">` + "\n")
	b.WriteString(`<meta http-equiv="X-UA-Compatible" content="IE=edge">` + "\n")
	fmt.Fprintf(&b, "<title>Charging session #%d</title>\n", tx.TransactionId)
	// Outlook ignores the font stack on block elements unless told otherwise.
	b.WriteString("<!--[if mso]>\n<style>table,td,div,p{font-family:Arial,Helvetica,sans-serif !important;}</style>\n<![endif]-->\n")
	fmt.Fprintf(&b, "</head>\n<body style=\"margin:0;padding:0;background-color:%s;-webkit-text-size-adjust:100%%;-ms-text-size-adjust:100%%;\">\n", colPage)

	// Preheader: the preview line inbox lists show next to the subject.
	preheader := fmt.Sprintf("Session #%d", tx.TransactionId)
	if energyKnown(tx) {
		preheader += " · " + formatEnergy(consumed)
	}
	if a := billedAmount(tx); a > 0 {
		preheader += " · " + formatAmount(a, currency)
	}
	preheader += " · " + stripTags(status.text)
	fmt.Fprintf(&b,
		`<div style="display:none;max-height:0;overflow:hidden;opacity:0;font-size:1px;line-height:1px;color:%s;">%s</div>`+"\n",
		colPage, html.EscapeString(preheader))

	fmt.Fprintf(&b, `<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:%s;">`, colPage)
	b.WriteString(`<tr><td align="center" style="padding:24px 12px;">`)
	fmt.Fprintf(&b,
		`<table role="presentation" width="600" cellpadding="0" cellspacing="0" border="0" `+
			`style="width:600px;max-width:600px;background-color:#ffffff;border:1px solid %s;border-radius:12px;overflow:hidden;font-family:%s;">`,
		colBorder, fontStack)

	// Header: title, context subline, status pill.
	subline := tx.ChargePointId
	if t.ChargePointTitle != "" {
		subline += " · " + t.ChargePointTitle
	}
	subline += " · " + formatShortTime(tx.TimeStart)

	fmt.Fprintf(&b, `<tr><td style="padding:20px 24px;border-bottom:1px solid %s;">`, colBorder)
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td valign="middle">`)
	fmt.Fprintf(&b, `<div style="font-size:18px;font-weight:bold;color:%s;line-height:1.3;">Charging session #%d</div>`, colText, tx.TransactionId)
	fmt.Fprintf(&b, `<div style="font-size:13px;color:%s;line-height:1.5;margin-top:2px;">%s</div>`, colMuted, html.EscapeString(subline))
	b.WriteString(`</td><td valign="middle" align="right" style="white-space:nowrap;">`)
	fmt.Fprintf(&b,
		`<span style="display:inline-block;font-size:12px;font-weight:bold;color:%s;background-color:%s;padding:5px 12px;border-radius:20px;">%s</span>`,
		status.fg, status.bg, status.text)
	b.WriteString(`</td></tr></table></td></tr>`)

	// KPI row. Unknown metrics render as an em dash rather than a false zero.
	energyValue, energyUnit := "—", ""
	if energyKnown(tx) {
		energyValue, energyUnit = formatEnergyHeadline(consumed), "kWh"
	}
	amountValue, amountUnit := "—", ""
	if a := billedAmount(tx); a > 0 {
		amountValue, amountUnit = fmt.Sprintf("%.2f", float64(a)/100.0), currency
	}

	b.WriteString(`<tr><td style="padding:0;"><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr>`)
	kpi(&b, "Energy", energyValue, energyUnit, true)
	kpi(&b, "Amount", amountValue, amountUnit, true)
	kpi(&b, "Duration", formatDuration(transactionDuration(tx)), "", false)
	b.WriteString(`</tr></table></td></tr>`)

	// Two detail columns.
	b.WriteString(`<tr><td style="padding:16px 24px 20px 24px;">`)
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr>`)
	b.WriteString(`<td width="50%" valign="top" style="padding-right:14px;">`)
	renderColumn(&b, []section{cp, sess, tariff})
	b.WriteString(`</td><td width="50%" valign="top" style="padding-left:14px;">`)
	renderColumn(&b, []section{user, pay, method})
	b.WriteString(`</td></tr></table></td></tr>`)

	// Full-width session strip.
	items := []string{stripItem("Started", formatTime(tx.TimeStart))}
	if tx.IsFinished {
		items = append(items, stripItem("Stopped", formatTime(tx.TimeStop)))
	}
	// A running session has no stop reading yet; showing "→ 0.000" would read as
	// a meter that ran backwards.
	if tx.MeterStop > 0 {
		// A literal arrow, not "&rarr;": stripItem escapes its value.
		items = append(items, stripItem("Meter",
			fmt.Sprintf("%s → %s", formatEnergyValue(tx.MeterStart), formatEnergy(tx.MeterStop))))
	} else {
		items = append(items, stripItem("Meter start", formatEnergy(tx.MeterStart)))
	}

	fmt.Fprintf(&b, `<tr><td style="padding:0 24px 20px 24px;">`)
	fmt.Fprintf(&b, `<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:%s;border-radius:8px;"><tr>`, colStrip)
	fmt.Fprintf(&b, `<td style="padding:12px 16px;font-size:12px;color:%s;line-height:1.6;">%s</td>`,
		colMuted, strings.Join(items, `&nbsp;&middot;&nbsp;`))
	b.WriteString(`</tr></table></td></tr>`)

	// Payment orders, full width, only when the session has any.
	if len(tx.PaymentOrders) > 0 {
		fmt.Fprintf(&b, `<tr><td style="padding:0 24px 20px 24px;">`)
		fmt.Fprintf(&b,
			`<div style="font-size:11px;font-weight:bold;text-transform:uppercase;letter-spacing:0.4px;color:%s;padding-bottom:6px;">Payment orders</div>`,
			colLabel)
		fmt.Fprintf(&b, `<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="font-size:12px;">`)
		fmt.Fprintf(&b, `<tr>`+
			`<td style="color:%s;padding:4px 0;border-bottom:1px solid %s;">Order</td>`+
			`<td align="right" style="color:%s;padding:4px 0;border-bottom:1px solid %s;">Amount</td>`+
			`<td align="right" style="color:%s;padding:4px 0;border-bottom:1px solid %s;">Result</td>`+
			`<td align="right" style="color:%s;padding:4px 0;border-bottom:1px solid %s;">Closed</td></tr>`,
			colLabel, colBorder, colLabel, colBorder, colLabel, colBorder, colLabel, colBorder)

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
			fmt.Fprintf(&b, `<tr>`+
				`<td style="color:%s;font-weight:bold;padding:5px 0;border-bottom:1px solid %s;">#%d</td>`+
				`<td align="right" style="color:%s;font-weight:bold;padding:5px 0;border-bottom:1px solid %s;">%s</td>`+
				`<td align="right" style="color:%s;padding:5px 0;border-bottom:1px solid %s;">%s</td>`+
				`<td align="right" style="color:%s;padding:5px 0;border-bottom:1px solid %s;">%s</td></tr>`,
				colText, colRule, o.Order,
				colText, colRule, html.EscapeString(amount),
				colMuted, colRule, html.EscapeString(result),
				colMuted, colRule, html.EscapeString(formatTime(o.TimeClosed)))
		}
		b.WriteString(`</table></td></tr>`)
	}

	// Footer.
	footer := fmt.Sprintf("Transaction %d", tx.TransactionId)
	if n := len(tx.PaymentOrders); n > 0 {
		last := tx.PaymentOrders[n-1]
		footer += fmt.Sprintf(" · Order #%d", last.Order)
		// An order that never settled has no close time; saying "closed —" reads
		// as though it did.
		if !last.TimeClosed.IsZero() {
			footer += " closed " + formatTime(last.TimeClosed)
		}
		if last.Result != "" {
			footer += fmt.Sprintf(" (result %s)", last.Result)
		}
	}
	footer += " · Wattbrews"

	fmt.Fprintf(&b, `<tr><td style="padding:14px 24px;border-top:1px solid %s;background-color:%s;">`, colBorder, colFooterBg)
	fmt.Fprintf(&b, `<div style="font-size:11px;color:%s;line-height:1.5;">%s</div>`, colLabel, html.EscapeString(footer))
	b.WriteString(`</td></tr>`)

	b.WriteString(`</table></td></tr></table>` + "\n</body>\n</html>\n")
	return b.String()
}

// stripTags removes the entities used in pill markup so the preheader, which is
// escaped as plain text, does not show "&#10003;" literally.
func stripTags(s string) string {
	s = strings.ReplaceAll(s, "&#10003;", "")
	s = strings.ReplaceAll(s, "&middot;", "·")
	return strings.TrimSpace(s)
}
