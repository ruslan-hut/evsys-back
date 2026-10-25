package aireport

import (
	"evsys-back/entity"
	"fmt"
	"html"
	"strings"
	"time"
)

// palette of impl/mail, so that the system's emails look alike
const (
	colText     = "#1a1a1a"
	colMuted    = "#6b6f76"
	colLabel    = "#9a9ea6"
	colBorder   = "#e3e5e8"
	colRule     = "#eeeff1"
	colPage     = "#f2f3f5"
	colStrip    = "#f7f8f9"
	colFooterBg = "#fafafb"
	fontStack   = "Arial,Helvetica,sans-serif"
)

type texts struct {
	title, sessions, energy, uptime, online, findings, trends, recommendation,
	noFindings, network, generated, subject string
	status   map[string]string
	severity map[string]string
}

var i18n = map[string]texts{
	entity.MailLanguageES: {
		title:          "Análisis semanal de EVSys",
		sessions:       "Sesiones",
		energy:         "Energía",
		uptime:         "Disponibilidad media",
		online:         "En línea ahora",
		findings:       "Hallazgos",
		trends:         "Tendencias",
		recommendation: "Recomendación",
		noFindings:     "No se han detectado problemas en este periodo.",
		network:        "Toda la red",
		generated:      "Generado automáticamente por IA (%s) a partir de los datos de EVSys. Compruebe los datos antes de actuar.",
		subject:        "EVSys · análisis semanal %s – %s: %s",
		status:         map[string]string{"ok": "Todo en orden", "attention": "Requiere atención", "critical": "Crítico"},
		severity:       map[string]string{"critical": "Crítico", "warning": "Aviso", "info": "Info"},
	},
	entity.MailLanguageEN: {
		title:          "EVSys weekly analysis",
		sessions:       "Sessions",
		energy:         "Energy",
		uptime:         "Average uptime",
		online:         "Online now",
		findings:       "Findings",
		trends:         "Trends",
		recommendation: "Recommendation",
		noFindings:     "No problems were found in this period.",
		network:        "Whole network",
		generated:      "Generated automatically by AI (%s) from EVSys data. Check the data before acting.",
		subject:        "EVSys · weekly analysis %s – %s: %s",
		status:         map[string]string{"ok": "All good", "attention": "Needs attention", "critical": "Critical"},
		severity:       map[string]string{"critical": "Critical", "warning": "Warning", "info": "Info"},
	},
}

func textsFor(language string) texts {
	if t, ok := i18n[language]; ok {
		return t
	}
	return i18n[entity.MailLanguageES]
}

type pill struct{ fg, bg string }

var statusPills = map[string]pill{
	"ok":        {"#0f6e56", "#e1f5ee"},
	"attention": {"#8a5a00", "#fdf3dc"},
	"critical":  {"#b00020", "#fdecef"},
}

var severityPills = map[string]pill{
	"critical": {"#b00020", "#fdecef"},
	"warning":  {"#8a5a00", "#fdf3dc"},
	"info":     {"#0b5cad", "#e3f0fb"},
}

// periodLabels formats the period as the days it covers, in the report's time
// zone: the end is exclusive, so the last day shown is the one before it.
func periodLabels(report *entity.AIReport) (string, string) {
	loc, err := time.LoadLocation(report.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	from := report.PeriodFrom.In(loc)
	last := report.PeriodTo.In(loc).Add(-time.Second)
	return from.Format("02.01"), last.Format("02.01.2006")
}

func subjectOf(report *entity.AIReport, result *entity.AIAnalysisResult, language string) string {
	t := textsFor(language)
	from, to := periodLabels(report)
	return fmt.Sprintf(t.subject, from, to, t.status[result.Status])
}

// renderReport builds the email of one analysis. snap may be nil when the
// stored snapshot cannot be read; the figures row is then left out.
func renderReport(report *entity.AIReport, analysis *entity.AIAnalysis, snap *Snapshot) string {
	t := textsFor(analysis.Language)
	result := analysis.Result
	status, ok := statusPills[result.Status]
	if !ok {
		status = statusPills["attention"]
	}
	from, to := periodLabels(report)

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n")
	fmt.Fprintf(&b, `<html lang="%s" xmlns="http://www.w3.org/1999/xhtml">`+"\n<head>\n", html.EscapeString(analysis.Language))
	b.WriteString(`<meta charset="utf-8">` + "\n")
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1.0">` + "\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(t.title))
	b.WriteString("<!--[if mso]>\n<style>table,td,div,p{font-family:Arial,Helvetica,sans-serif !important;}</style>\n<![endif]-->\n")
	fmt.Fprintf(&b, "</head>\n<body style=\"margin:0;padding:0;background-color:%s;-webkit-text-size-adjust:100%%;-ms-text-size-adjust:100%%;\">\n", colPage)

	fmt.Fprintf(&b,
		`<div style="display:none;max-height:0;overflow:hidden;opacity:0;font-size:1px;line-height:1px;color:%s;">%s</div>`+"\n",
		colPage, html.EscapeString(result.Headline))

	fmt.Fprintf(&b, `<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:%s;">`, colPage)
	b.WriteString(`<tr><td align="center" style="padding:24px 12px;">`)
	fmt.Fprintf(&b,
		`<table role="presentation" width="600" cellpadding="0" cellspacing="0" border="0" `+
			`style="width:600px;max-width:600px;background-color:#ffffff;border:1px solid %s;border-radius:12px;overflow:hidden;font-family:%s;">`,
		colBorder, fontStack)

	// header: title, period, status pill
	fmt.Fprintf(&b, `<tr><td style="padding:20px 24px;border-bottom:1px solid %s;">`, colBorder)
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td valign="middle">`)
	fmt.Fprintf(&b, `<div style="font-size:18px;font-weight:bold;color:%s;line-height:1.3;">%s</div>`, colText, html.EscapeString(t.title))
	fmt.Fprintf(&b, `<div style="font-size:13px;color:%s;line-height:1.5;margin-top:2px;">%s – %s</div>`, colMuted, from, to)
	b.WriteString(`</td><td valign="middle" align="right" style="white-space:nowrap;">`)
	fmt.Fprintf(&b,
		`<span style="display:inline-block;font-size:12px;font-weight:bold;color:%s;background-color:%s;padding:5px 12px;border-radius:20px;">%s</span>`,
		status.fg, status.bg, html.EscapeString(t.status[result.Status]))
	b.WriteString(`</td></tr></table></td></tr>`)

	// headline and summary
	fmt.Fprintf(&b, `<tr><td style="padding:18px 24px 6px 24px;">`)
	fmt.Fprintf(&b, `<div style="font-size:15px;font-weight:bold;color:%s;line-height:1.4;">%s</div>`, colText, html.EscapeString(result.Headline))
	fmt.Fprintf(&b, `<div style="font-size:13px;color:%s;line-height:1.6;margin-top:8px;">%s</div>`, colText, paragraphs(result.Summary))
	b.WriteString(`</td></tr>`)

	if snap != nil {
		renderFigures(&b, snap, t)
	}

	// findings
	fmt.Fprintf(&b, `<tr><td style="padding:16px 24px 4px 24px;">`)
	caption(&b, t.findings)
	if len(result.Findings) == 0 {
		fmt.Fprintf(&b, `<div style="font-size:13px;color:%s;padding:6px 0 10px 0;">%s</div>`, colMuted, html.EscapeString(t.noFindings))
	}
	for _, f := range result.Findings {
		renderFinding(&b, f, t)
	}
	b.WriteString(`</td></tr>`)

	if len(result.Trends) > 0 {
		fmt.Fprintf(&b, `<tr><td style="padding:8px 24px 18px 24px;">`)
		caption(&b, t.trends)
		fmt.Fprintf(&b, `<ul style="margin:6px 0 0 0;padding-left:18px;font-size:13px;color:%s;line-height:1.6;">`, colText)
		for _, tr := range result.Trends {
			fmt.Fprintf(&b, `<li>%s</li>`, html.EscapeString(tr))
		}
		b.WriteString(`</ul></td></tr>`)
	}

	footer := fmt.Sprintf(t.generated, report.Model)
	fmt.Fprintf(&b, `<tr><td style="padding:14px 24px;border-top:1px solid %s;background-color:%s;">`, colBorder, colFooterBg)
	fmt.Fprintf(&b, `<div style="font-size:11px;color:%s;line-height:1.5;">%s · %s</div>`,
		colLabel, html.EscapeString(footer), html.EscapeString(report.Id))
	b.WriteString(`</td></tr>`)

	b.WriteString(`</table></td></tr></table>` + "\n</body>\n</html>\n")
	return b.String()
}

func caption(b *strings.Builder, text string) {
	fmt.Fprintf(b,
		`<div style="font-size:11px;font-weight:bold;text-transform:uppercase;letter-spacing:0.4px;color:%s;padding-bottom:4px;">%s</div>`,
		colLabel, html.EscapeString(text))
}

func paragraphs(text string) string {
	parts := strings.Split(strings.TrimSpace(text), "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, html.EscapeString(p))
		}
	}
	return strings.Join(out, "<br>")
}

// renderFigures writes the row of figures computed from the data, not by the
// model, so that the numbers in the email are exact.
func renderFigures(b *strings.Builder, snap *Snapshot, t texts) {
	b.WriteString(`<tr><td style="padding:12px 0 0 0;"><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr>`)
	if s := snap.Sessions; s != nil {
		figure(b, t.sessions, fmt.Sprintf("%d", s.Finished), "", change(float64(s.Finished), float64(s.PrevFinished)), true)
		figure(b, t.energy, fmt.Sprintf("%.1f", s.EnergyKWh), "kWh", change(s.EnergyKWh, s.PrevEnergyKWh), true)
	}
	if avg, prev, ok := averageUptime(snap.Stations); ok {
		diff := ""
		if prev > 0 {
			diff = fmt.Sprintf("%+.1f p.p.", avg-prev)
		}
		figure(b, t.uptime, fmt.Sprintf("%.1f", avg), "%", diff, true)
	}
	if n := snap.Network; n != nil {
		figure(b, t.online, fmt.Sprintf("%d/%d", n.Online, n.Enabled), "", "", false)
	}
	b.WriteString(`</tr></table></td></tr>`)
}

func figure(b *strings.Builder, label, value, unit, diff string, rightRule bool) {
	rule := ""
	if rightRule {
		rule = fmt.Sprintf("border-right:1px solid %s;", colBorder)
	}
	unitSpan := ""
	if unit != "" {
		unitSpan = fmt.Sprintf(` <span style="font-size:12px;font-weight:normal;color:%s;">%s</span>`, colMuted, html.EscapeString(unit))
	}
	diffDiv := ""
	if diff != "" {
		diffDiv = fmt.Sprintf(`<div style="font-size:11px;color:%s;margin-top:2px;">%s</div>`, colMuted, html.EscapeString(diff))
	}
	fmt.Fprintf(b,
		`<td width="25%%" valign="top" style="padding:12px 16px;border-top:1px solid %s;border-bottom:1px solid %s;background-color:%s;%s">`+
			`<div style="font-size:11px;color:%s;">%s</div>`+
			`<div style="font-size:20px;font-weight:bold;color:%s;line-height:1.2;margin-top:2px;">%s%s</div>%s</td>`,
		colBorder, colBorder, colStrip, rule, colMuted, html.EscapeString(label), colText, html.EscapeString(value), unitSpan, diffDiv)
}

func change(cur, prev float64) string {
	if prev <= 0 {
		return ""
	}
	return fmt.Sprintf("%+.0f %%", (cur-prev)/prev*100)
}

func averageUptime(stations []*StationWeek) (float64, float64, bool) {
	var sum, prevSum float64
	var n, prevN int
	for _, st := range stations {
		if st.UptimePercent != nil {
			sum += *st.UptimePercent
			n++
		}
		if st.PrevUptimePercent != nil {
			prevSum += *st.PrevUptimePercent
			prevN++
		}
	}
	if n == 0 {
		return 0, 0, false
	}
	prev := 0.0
	if prevN > 0 {
		prev = prevSum / float64(prevN)
	}
	return sum / float64(n), prev, true
}

func renderFinding(b *strings.Builder, f *entity.AIFinding, t texts) {
	p, ok := severityPills[f.Severity]
	if !ok {
		p = severityPills["info"]
	}
	target := f.ChargePointId
	if target == "" {
		target = t.network
	}
	fmt.Fprintf(b, `<div style="padding:10px 0 12px 0;border-bottom:1px solid %s;">`, colRule)
	fmt.Fprintf(b,
		`<div><span style="display:inline-block;font-size:11px;font-weight:bold;color:%s;background-color:%s;padding:2px 8px;border-radius:10px;">%s</span>`+
			` <span style="font-size:12px;color:%s;">%s</span></div>`,
		p.fg, p.bg, html.EscapeString(t.severity[f.Severity]), colMuted, html.EscapeString(target))
	fmt.Fprintf(b, `<div style="font-size:14px;font-weight:bold;color:%s;line-height:1.4;margin-top:6px;">%s</div>`, colText, html.EscapeString(f.Title))
	fmt.Fprintf(b, `<div style="font-size:13px;color:%s;line-height:1.6;margin-top:4px;">%s</div>`, colText, paragraphs(f.Details))
	if f.Recommendation != "" {
		fmt.Fprintf(b,
			`<div style="font-size:13px;color:%s;line-height:1.6;margin-top:6px;padding:8px 12px;background-color:%s;border-radius:6px;">`+
				`<strong style="color:%s;">%s:</strong> %s</div>`,
			colText, colStrip, colText, html.EscapeString(t.recommendation), html.EscapeString(f.Recommendation))
	}
	b.WriteString(`</div>`)
}
