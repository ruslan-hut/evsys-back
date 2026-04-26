package mail

import (
	"context"
	"encoding/json"
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"fmt"
	"html"
	"log/slog"
	"sort"
	"strings"
	"time"
)

// Repository is the narrow database surface the mail service needs.
type Repository interface {
	ListMailSubscriptionsByPeriod(ctx context.Context, period string) ([]*entity.MailSubscription, error)
}

// ReportSource produces aggregated session totals grouped by charger.
type ReportSource interface {
	TotalsByCharger(ctx context.Context, from, to time.Time, userGroup string) ([]interface{}, error)
}

// Sender delivers a single transactional email.
type Sender interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}

// Service schedules and dispatches periodic statistic emails.
type Service struct {
	repo    Repository
	reports ReportSource
	sender  Sender
	log     *slog.Logger
	stop    chan struct{}
	now     func() time.Time
}

func New(repo Repository, reports ReportSource, sender Sender, log *slog.Logger) *Service {
	return &Service{
		repo:    repo,
		reports: reports,
		sender:  sender,
		log:     log.With(sl.Module("impl.mail")),
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// Start launches the scheduler goroutine. It fires once per day at 06:00 UTC
// and sends the periods (daily / weekly / monthly) that are due that day.
func (s *Service) Start() {
	if s.stop != nil {
		return
	}
	s.stop = make(chan struct{})
	go s.loop()
	s.log.Info("mail scheduler started")
}

// Stop signals the scheduler goroutine to exit.
func (s *Service) Stop() {
	if s.stop == nil {
		return
	}
	close(s.stop)
	s.stop = nil
}

func (s *Service) loop() {
	for {
		next := nextTick(s.now())
		wait := time.Until(next)
		s.log.Debug("mail scheduler sleep",
			slog.Time("until", next),
			slog.Duration("wait", wait),
		)
		select {
		case <-s.stop:
			s.log.Info("mail scheduler stopped")
			return
		case <-time.After(wait):
		}
		s.runDuePeriods(context.Background(), s.now())
	}
}

// nextTick returns the next 06:00 UTC strictly after `from`.
func nextTick(from time.Time) time.Time {
	from = from.UTC()
	tick := time.Date(from.Year(), from.Month(), from.Day(), 6, 0, 0, 0, time.UTC)
	if !from.Before(tick) {
		tick = tick.Add(24 * time.Hour)
	}
	return tick
}

func (s *Service) runDuePeriods(ctx context.Context, now time.Time) {
	now = now.UTC()
	periods := []string{entity.MailPeriodDaily}
	if now.Weekday() == time.Monday {
		periods = append(periods, entity.MailPeriodWeekly)
	}
	if now.Day() == 1 {
		periods = append(periods, entity.MailPeriodMonthly)
	}
	for _, p := range periods {
		if err := s.RunOnce(ctx, p); err != nil {
			s.log.Error("scheduled mail run failed",
				slog.String("period", p),
				sl.Err(err),
			)
		}
	}
}

// RunOnce loads all enabled subscriptions for the given period, builds a report
// per (group, period) once, and emails each subscriber. Per-recipient errors
// are logged but do not abort the batch.
func (s *Service) RunOnce(ctx context.Context, period string) error {
	subs, err := s.repo.ListMailSubscriptionsByPeriod(ctx, period)
	if err != nil {
		return fmt.Errorf("list subscriptions: %w", err)
	}
	if len(subs) == 0 {
		s.log.Debug("no subscriptions for period", slog.String("period", period))
		return nil
	}

	from, to := periodRange(s.now(), period)

	type cacheKey struct{ group string }
	cache := make(map[cacheKey][]chargerLine)

	for _, sub := range subs {
		key := cacheKey{group: sub.UserGroup}
		lines, ok := cache[key]
		if !ok {
			raw, err := s.reports.TotalsByCharger(ctx, from, to, sub.UserGroup)
			if err != nil {
				s.log.Error("load report failed",
					slog.String("group", sub.UserGroup),
					sl.Err(err),
				)
				continue
			}
			lines = parseChargerLines(raw)
			cache[key] = lines
		}

		body := renderHTML(period, sub.UserGroup, from, to, lines)
		subject := buildSubject(period, from, to)
		if err := s.sender.Send(ctx, sub.Email, subject, body); err != nil {
			s.log.Error("send mail failed",
				slog.String("to", sub.Email),
				slog.String("period", period),
				sl.Err(err),
			)
			continue
		}
		s.log.Info("report mail sent",
			slog.String("to", sub.Email),
			slog.String("period", period),
			slog.String("group", sub.UserGroup),
		)
	}
	return nil
}

// SendTest sends a minimal diagnostic email to the given address. It exercises
// the Brevo client (credentials, sender verification, network) without
// touching the report repository, so admins can validate the mail pipeline
// before any subscription exists.
func (s *Service) SendTest(ctx context.Context, to string) error {
	now := s.now()
	subject := "EVSys mail test"
	body := fmt.Sprintf(
		`<!DOCTYPE html><html><body style="font-family:Arial,sans-serif;color:#222;">`+
			`<h2>Mail integration is working</h2>`+
			`<p>This is a test message sent from the EVSys backend to verify the Brevo configuration.</p>`+
			`<p style="color:#666;">Sent at %s UTC.</p>`+
			`</body></html>`,
		now.UTC().Format("2006-01-02 15:04:05"),
	)
	return s.sender.Send(ctx, to, subject, body)
}

// SendNow runs the report for a single subscription immediately, useful as an
// admin "test" action. It honours the subscription's period to compute the date
// range, but ignores the Enabled flag.
func (s *Service) SendNow(ctx context.Context, sub *entity.MailSubscription) error {
	from, to := periodRange(s.now(), sub.Period)
	raw, err := s.reports.TotalsByCharger(ctx, from, to, sub.UserGroup)
	if err != nil {
		return fmt.Errorf("load report: %w", err)
	}
	lines := parseChargerLines(raw)
	body := renderHTML(sub.Period, sub.UserGroup, from, to, lines)
	subject := buildSubject(sub.Period, from, to)
	return s.sender.Send(ctx, sub.Email, subject, body)
}

// periodRange returns [from, to) covering the previous full period.
// daily: yesterday 00:00 → today 00:00.
// weekly: previous Mon 00:00 → this Mon 00:00.
// monthly: 1st of previous month 00:00 → 1st of this month 00:00.
func periodRange(now time.Time, period string) (time.Time, time.Time) {
	now = now.UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch period {
	case entity.MailPeriodWeekly:
		// Days back to most recent Monday (today included if Monday).
		offset := int(today.Weekday()) - int(time.Monday)
		if offset < 0 {
			offset += 7
		}
		thisMonday := today.AddDate(0, 0, -offset)
		return thisMonday.AddDate(0, 0, -7), thisMonday
	case entity.MailPeriodMonthly:
		firstThisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		return firstThisMonth.AddDate(0, -1, 0), firstThisMonth
	default: // daily
		return today.AddDate(0, 0, -1), today
	}
}

type chargerLine struct {
	User    string  `json:"user"`
	Total   int64   `json:"total"`
	Count   int64   `json:"count"`
	Average float64 `json:"average"`
}

// parseChargerLines defensively converts the report payload (typed as
// []interface{} at the source) into a typed slice via JSON round-trip,
// avoiding cross-package coupling with the database layer.
func parseChargerLines(raw []interface{}) []chargerLine {
	if len(raw) == 0 {
		return nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var lines []chargerLine
	if err := json.Unmarshal(buf, &lines); err != nil {
		return nil
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].User < lines[j].User })
	return lines
}

func buildSubject(period string, from, to time.Time) string {
	end := to.Add(-time.Second)
	switch period {
	case entity.MailPeriodWeekly:
		return fmt.Sprintf("Weekly charging report — %s to %s",
			from.Format("2006-01-02"), end.Format("2006-01-02"))
	case entity.MailPeriodMonthly:
		return fmt.Sprintf("Monthly charging report — %s", from.Format("January 2006"))
	default:
		return fmt.Sprintf("Daily charging report — %s", from.Format("2006-01-02"))
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func renderHTML(period, group string, from, to time.Time, lines []chargerLine) string {
	end := to.Add(-time.Second)
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html><body style="font-family:Arial,sans-serif;color:#222;">`)
	fmt.Fprintf(&b, `<h2 style="margin-bottom:4px;">%s charging report</h2>`,
		html.EscapeString(capitalize(period)))
	fmt.Fprintf(&b, `<p style="color:#666;margin-top:0;">Period: %s &mdash; %s · Client: <strong>%s</strong></p>`,
		from.Format("2006-01-02"), end.Format("2006-01-02"), html.EscapeString(group))

	if len(lines) == 0 {
		b.WriteString(`<p>No charging sessions in this period.</p>`)
		b.WriteString(`</body></html>`)
		return b.String()
	}

	b.WriteString(`<table cellpadding="6" cellspacing="0" border="0" style="border-collapse:collapse;min-width:480px;">`)
	b.WriteString(`<thead><tr style="background:#f3f3f3;text-align:left;">`)
	b.WriteString(`<th style="border-bottom:1px solid #ccc;">Charger</th>`)
	b.WriteString(`<th style="border-bottom:1px solid #ccc;text-align:right;">Sessions</th>`)
	b.WriteString(`<th style="border-bottom:1px solid #ccc;text-align:right;">Energy (kWh)</th>`)
	b.WriteString(`<th style="border-bottom:1px solid #ccc;text-align:right;">Avg power (kW)</th>`)
	b.WriteString(`</tr></thead><tbody>`)

	var totalCount int64
	var totalWh int64
	for _, l := range lines {
		fmt.Fprintf(&b,
			`<tr><td style="border-bottom:1px solid #eee;">%s</td>`+
				`<td style="border-bottom:1px solid #eee;text-align:right;">%d</td>`+
				`<td style="border-bottom:1px solid #eee;text-align:right;">%.2f</td>`+
				`<td style="border-bottom:1px solid #eee;text-align:right;">%.2f</td></tr>`,
			html.EscapeString(l.User), l.Count, float64(l.Total)/1000.0, l.Average/1000.0)
		totalCount += l.Count
		totalWh += l.Total
	}
	fmt.Fprintf(&b,
		`<tr style="font-weight:bold;background:#fafafa;">`+
			`<td>Total</td>`+
			`<td style="text-align:right;">%d</td>`+
			`<td style="text-align:right;">%.2f</td>`+
			`<td></td></tr>`,
		totalCount, float64(totalWh)/1000.0)
	b.WriteString(`</tbody></table>`)
	b.WriteString(`</body></html>`)
	return b.String()
}
