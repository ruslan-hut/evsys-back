package aireport

import (
	"context"
	"encoding/json"
	"errors"
	"evsys-back/entity"
	databasemock "evsys-back/impl/database-mock"
	"evsys-back/impl/openai"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeModel struct {
	mu         sync.Mutex
	configured bool
	err        error
	requests   []openai.Request
}

func (m *fakeModel) Configured() bool { return m.configured }

func (m *fakeModel) Respond(_ context.Context, req openai.Request) (*openai.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, req)
	resp := &openai.Response{
		Id:     "resp_1",
		Model:  req.Model,
		Status: "completed",
		Usage:  openai.Usage{InputTokens: 40_000, CachedTokens: 10_000, OutputTokens: 3_000, ReasoningTokens: 1_000},
	}
	if m.err != nil {
		return resp, m.err
	}
	lang := "es"
	if strings.Contains(req.Input, "in English") {
		lang = "en"
	}
	data, _ := json.Marshal(entity.AIAnalysisResult{
		Status:   "attention",
		Headline: "headline " + lang,
		Summary:  "summary " + lang,
		Findings: []*entity.AIFinding{
			{Severity: "info", Area: "other", Title: "minor"},
			{Severity: "critical", Area: "hardware", ChargePointId: "PE00004", Title: "faulted"},
		},
		Trends: []string{"trend"},
	})
	resp.Text = string(data)
	return resp, nil
}

func (m *fakeModel) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requests)
}

type sentMail struct{ to, subject, body string }

type fakeSender struct {
	mu   sync.Mutex
	sent []sentMail
}

func (s *fakeSender) Send(_ context.Context, to, subject, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, sentMail{to, subject, body})
	return nil
}

func (s *fakeSender) all() []sentMail {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]sentMail(nil), s.sent...)
}

func newTestService(t *testing.T, now time.Time) (*Service, *databasemock.MockDB, *fakeModel, *fakeSender) {
	t.Helper()
	db := databasemock.NewMockDB()
	model := &fakeModel{configured: true}
	sender := &fakeSender{}
	s := New(db, model, sender, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.now = func() time.Time { return now }
	return s, db, model, sender
}

func subscribe(t *testing.T, db *databasemock.MockDB, email, language string, enabled bool) {
	t.Helper()
	sub := &entity.MailSubscription{Email: email, Report: entity.MailReportAIAnalysis, Language: language, Enabled: enabled}
	sub.Normalize()
	if _, err := db.SaveMailSubscription(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
}

// wait blocks until the background run releases the service.
func wait(s *Service) {
	s.running.Lock()
	s.running.Unlock()
}

func TestTickRunsScheduledSlotOnce(t *testing.T) {
	ctx := context.Background()
	// Monday 2026-11-02 07:20 in Madrid
	s, db, model, sender := newTestService(t, mustUTC(t, "2026-11-02T06:20:00Z"))
	subscribe(t, db, "es@example.com", "es", true)
	subscribe(t, db, "en@example.com", "en", true)
	subscribe(t, db, "off@example.com", "en", false)

	s.tick()

	if got := model.calls(); got != 2 {
		t.Fatalf("model calls = %d, want one per language (2)", got)
	}
	// the second language reuses the snapshot prefix for the prompt cache
	first, second := model.requests[0].Input, model.requests[1].Input
	prefix := first[:strings.Index(first, "Write every text field")]
	if !strings.HasPrefix(second, prefix) {
		t.Error("requests of one run must share the snapshot prefix")
	}

	mails := sender.all()
	if len(mails) != 2 {
		t.Fatalf("sent %d mails, want 2", len(mails))
	}
	for _, m := range mails {
		switch m.to {
		case "es@example.com":
			if !strings.Contains(m.subject, "análisis semanal 26.10 – 01.11.2026") || !strings.Contains(m.body, "headline es") {
				t.Errorf("spanish mail: subject %q", m.subject)
			}
		case "en@example.com":
			if !strings.Contains(m.subject, "weekly analysis") || !strings.Contains(m.body, "headline en") {
				t.Errorf("english mail: subject %q", m.subject)
			}
		default:
			t.Errorf("mail to %s, a disabled subscriber", m.to)
		}
	}

	reports, _ := db.ListAIReports(ctx, 10)
	if len(reports) != 1 {
		t.Fatalf("reports = %d, want 1", len(reports))
	}
	r := reports[0]
	if r.Status != entity.AIReportDone || r.Trigger != entity.AITriggerSchedule {
		t.Errorf("report status %s trigger %s", r.Status, r.Trigger)
	}
	if !r.PeriodFrom.Equal(mustUTC(t, "2026-10-25T23:00:00Z")) || !r.PeriodTo.Equal(mustUTC(t, "2026-11-01T23:00:00Z")) {
		t.Errorf("period [%s, %s)", r.PeriodFrom, r.PeriodTo)
	}
	if r.Analyses[0].Result.Findings[0].Severity != "critical" {
		t.Error("findings must be sorted by severity")
	}
	// 30k uncached * 2 + 10k cached * 0.10 + 3k out * 10, per million, twice
	wantCost := 2 * (30_000*2.00 + 10_000*0.10 + 3_000*10.00) / 1_000_000
	if diff := r.CostUSD - wantCost; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("cost = %f, want %f", r.CostUSD, wantCost)
	}

	usage, _ := db.ListAIUsage(ctx, time.Time{}, mustUTC(t, "2030-01-01T00:00:00Z"))
	if len(usage) != 2 {
		t.Errorf("usage records = %d, want 2", len(usage))
	}

	// a later tick in the same slot does nothing
	s.now = func() time.Time { return mustUTC(t, "2026-11-02T06:40:00Z") }
	s.tick()
	if got := model.calls(); got != 2 {
		t.Errorf("model calls after a second tick = %d, want 2", got)
	}
}

func TestTickSkips(t *testing.T) {
	tests := []struct {
		name  string
		now   string
		setup func(s *Service, db *databasemock.MockDB, m *fakeModel)
	}{
		{"before the slot", "2026-11-02T05:50:00Z", nil},
		{"slot older than the catch-up window", "2026-11-02T12:30:00Z", nil},
		{"no api key", "2026-11-02T06:20:00Z", func(_ *Service, _ *databasemock.MockDB, m *fakeModel) { m.configured = false }},
		{"no mail", "2026-11-02T06:20:00Z", func(s *Service, _ *databasemock.MockDB, _ *fakeModel) { s.sender = nil }},
		{"disabled", "2026-11-02T06:20:00Z", func(_ *Service, db *databasemock.MockDB, _ *fakeModel) {
			st := entity.DefaultAIReportSettings()
			st.Enabled = false
			_ = db.SaveAIReportSettings(context.Background(), st)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, db, model, _ := newTestService(t, mustUTC(t, tt.now))
			subscribe(t, db, "es@example.com", "es", true)
			if tt.setup != nil {
				tt.setup(s, db, model)
			}
			s.tick()
			if got := model.calls(); got != 0 {
				t.Errorf("model calls = %d, want 0", got)
			}
		})
	}
}

func TestTickWithoutSubscribersClaimsTheSlot(t *testing.T) {
	ctx := context.Background()
	s, db, model, _ := newTestService(t, mustUTC(t, "2026-11-02T06:20:00Z"))
	s.tick()
	if model.calls() != 0 {
		t.Error("no subscribers, no request")
	}
	settings, _ := db.GetAIReportSettings(ctx)
	if settings == nil || settings.LastScheduledRun == nil || !settings.LastScheduledRun.Equal(mustUTC(t, "2026-11-02T06:00:00Z")) {
		t.Errorf("slot not claimed: %+v", settings)
	}
}

func TestRunDryRun(t *testing.T) {
	ctx := context.Background()
	s, _, model, sender := newTestService(t, mustUTC(t, "2026-11-04T10:00:00Z"))
	report, err := s.Run(ctx, "admin", &entity.AIReportRunRequest{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	wait(s)
	got, err := s.Report(ctx, report.Id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != entity.AIReportDone || len(got.Scopes) != 1 || got.Scopes[0].Snapshot == "" || got.SnapshotBytes == 0 {
		t.Errorf("dry run report: status %s, %d bytes", got.Status, got.SnapshotBytes)
	}
	if model.calls() != 0 || len(sender.all()) != 0 {
		t.Error("a dry run neither asks the model nor sends mail")
	}
}

func TestRunRefusals(t *testing.T) {
	tests := []struct {
		name  string
		req   entity.AIReportRunRequest
		setup func(s *Service, db *databasemock.MockDB, m *fakeModel)
		want  string
	}{
		{"no api key", entity.AIReportRunRequest{}, func(_ *Service, _ *databasemock.MockDB, m *fakeModel) { m.configured = false },
			"not configured"},
		{"send all without subscribers", entity.AIReportRunRequest{SendAll: true}, nil, "no enabled subscribers"},
		{"budget reached", entity.AIReportRunRequest{}, func(_ *Service, db *databasemock.MockDB, _ *fakeModel) {
			st := entity.DefaultAIReportSettings()
			st.MonthlyBudgetUSD = 1
			_ = db.SaveAIReportSettings(context.Background(), st)
			_ = db.SaveAIUsage(context.Background(), &entity.AIUsage{Id: "u", Time: mustUTC(t, "2026-11-03T10:00:00Z"), CostUSD: 1.2})
		}, "monthly budget"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, db, model, _ := newTestService(t, mustUTC(t, "2026-11-04T10:00:00Z"))
			if tt.setup != nil {
				tt.setup(s, db, model)
			}
			_, err := s.Run(context.Background(), "admin", &tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRunRecordsFailedRequest(t *testing.T) {
	ctx := context.Background()
	s, db, model, _ := newTestService(t, mustUTC(t, "2026-11-04T10:00:00Z"))
	model.err = errors.New("openai answer not complete (incomplete: max_output_tokens)")
	report, err := s.Run(ctx, "admin", &entity.AIReportRunRequest{Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	wait(s)
	got, _ := s.Report(ctx, report.Id)
	if got.Status != entity.AIReportFailed || !strings.Contains(got.Error, "max_output_tokens") {
		t.Errorf("status %s error %q", got.Status, got.Error)
	}
	usage, _ := db.ListAIUsage(ctx, time.Time{}, mustUTC(t, "2030-01-01T00:00:00Z"))
	if len(usage) != 1 || usage[0].Success || usage[0].CostUSD == 0 {
		t.Errorf("a failed request is still paid for and recorded: %+v", usage)
	}
}

func TestSendToReusesReportOfThePeriod(t *testing.T) {
	ctx := context.Background()
	s, db, model, sender := newTestService(t, mustUTC(t, "2026-11-04T10:00:00Z"))
	if _, err := s.Run(ctx, "admin", &entity.AIReportRunRequest{Language: "es"}); err != nil {
		t.Fatal(err)
	}
	wait(s)

	sub := &entity.MailSubscription{Email: "late@example.com", Report: entity.MailReportAIAnalysis, Language: "es", Enabled: true}
	if err := s.SendTo(ctx, "admin", sub); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(sender.all()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if model.calls() != 1 {
		t.Errorf("model calls = %d, want the stored analysis to be reused", model.calls())
	}
	if mails := sender.all(); len(mails) != 1 || mails[0].to != "late@example.com" {
		t.Errorf("mails = %+v", mails)
	}

	// a language the report lacks needs a new run
	sub.Language = "en"
	if err := s.SendTo(ctx, "admin", sub); err != nil {
		t.Fatal(err)
	}
	wait(s)
	if model.calls() != 2 {
		t.Errorf("model calls = %d, want a new run for english", model.calls())
	}
	_ = db
}

func TestUsageTotals(t *testing.T) {
	ctx := context.Background()
	s, db, _, _ := newTestService(t, mustUTC(t, "2026-11-04T10:00:00Z"))
	for i, u := range []*entity.AIUsage{
		{Time: mustUTC(t, "2026-10-12T06:00:00Z"), Model: "gpt-6.1-sol", InputTokens: 100, CostUSD: 0.10, Success: true},
		{Time: mustUTC(t, "2026-11-02T06:00:00Z"), Model: "gpt-6.1-sol", InputTokens: 200, CostUSD: 0.20, Success: true},
		{Time: mustUTC(t, "2026-11-03T06:00:00Z"), Model: "gpt-6-luna", InputTokens: 50, CostUSD: 0.01},
	} {
		u.Id = string(rune('a' + i))
		_ = db.SaveAIUsage(ctx, u)
	}
	report, err := s.Usage(ctx, mustUTC(t, "2026-10-01T00:00:00Z"), mustUTC(t, "2026-11-30T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals.Requests != 3 || report.Totals.Failed != 1 || report.Totals.InputTokens != 350 {
		t.Errorf("totals %+v", report.Totals)
	}
	if len(report.Months) != 2 || report.Months[0].Month != "2026-11" || report.Months[0].Requests != 2 {
		t.Errorf("months %+v", report.Months)
	}
	if report.Models["gpt-6.1-sol"].Requests != 2 {
		t.Errorf("models %+v", report.Models)
	}

	view, err := s.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if diff := view.MonthSpentUSD - 0.21; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("month spent = %f, want 0.21", view.MonthSpentUSD)
	}
	if view.NextRun == nil || !view.NextRun.Equal(mustUTC(t, "2026-11-09T06:00:00Z")) {
		t.Errorf("next run = %v", view.NextRun)
	}
}

func TestUpdateSettings(t *testing.T) {
	ctx := context.Background()
	s, db, _, _ := newTestService(t, mustUTC(t, "2026-11-04T10:00:00Z"))
	bad := "Mars/Olympus"
	if _, err := s.UpdateSettings(ctx, "admin", &entity.AIReportSettingsUpdate{TimeZone: &bad}); err == nil {
		t.Error("an unknown time zone must be refused")
	}
	unknown := "gpt-0"
	if _, err := s.UpdateSettings(ctx, "admin", &entity.AIReportSettingsUpdate{Model: &unknown}); err == nil {
		t.Error("a model without prices must be refused")
	}
	hour, model := 8, "gpt-6-luna"
	view, err := s.UpdateSettings(ctx, "admin", &entity.AIReportSettingsUpdate{Hour: &hour, Model: &model})
	if err != nil {
		t.Fatal(err)
	}
	if view.Hour != 8 || view.Model != "gpt-6-luna" || view.TimeZone != entity.AIReportDefaultTimeZone || view.UpdatedBy != "admin" {
		t.Errorf("settings %+v", view.AIReportSettings)
	}
	stored, _ := db.GetAIReportSettings(ctx)
	if stored == nil || stored.Hour != 8 {
		t.Error("settings not stored")
	}
}

func TestTickSkipsSlotBeforeSettingsChange(t *testing.T) {
	ctx := context.Background()
	// Monday 09:00 in Madrid; the admin enabled the analysis at 08:30
	s, db, model, _ := newTestService(t, mustUTC(t, "2026-11-02T08:00:00Z"))
	subscribe(t, db, "es@example.com", "es", true)
	st := entity.DefaultAIReportSettings()
	st.UpdatedAt = mustUTC(t, "2026-11-02T07:30:00Z")
	_ = db.SaveAIReportSettings(ctx, st)

	s.tick()
	if model.calls() != 0 {
		t.Error("the 07:00 slot passed before the settings were saved: no run")
	}

	// a restart within the window after a change made before the slot still catches up
	st.UpdatedAt = mustUTC(t, "2026-11-02T05:00:00Z")
	_ = db.SaveAIReportSettings(ctx, st)
	s.tick()
	if model.calls() != 1 {
		t.Errorf("model calls = %d, want the missed slot caught up", model.calls())
	}
}

func TestTickWaitsForManualRun(t *testing.T) {
	s, db, model, _ := newTestService(t, mustUTC(t, "2026-11-02T06:20:00Z"))
	subscribe(t, db, "es@example.com", "es", true)
	s.running.Lock()
	s.tick()
	s.running.Unlock()
	if model.calls() != 0 {
		t.Fatal("a busy service must not run the slot")
	}
	settings, _ := db.GetAIReportSettings(context.Background())
	if settings != nil && settings.LastScheduledRun != nil {
		t.Fatal("a skipped tick must not claim the slot")
	}
	s.tick()
	if model.calls() != 1 {
		t.Errorf("model calls = %d, want the slot run on the next tick", model.calls())
	}
}

func TestStartRecoversInterruptedRun(t *testing.T) {
	ctx := context.Background()
	s, db, model, _ := newTestService(t, mustUTC(t, "2026-11-02T06:30:00Z"))
	subscribe(t, db, "es@example.com", "es", true)
	slot := mustUTC(t, "2026-11-02T06:00:00Z")
	_ = db.SaveAIReportSettings(ctx, entity.DefaultAIReportSettings())
	if ok, _ := db.ClaimAIReportSlot(ctx, slot); !ok {
		t.Fatal("claim")
	}
	_ = db.SaveAIReport(ctx, &entity.AIReport{
		Id: "stale", Trigger: entity.AITriggerSchedule, Status: entity.AIReportRunning,
		Slot: &slot, CreatedAt: slot, TimeZone: entity.AIReportDefaultTimeZone,
	})

	s.Start()
	s.Stop()

	stale, _ := db.GetAIReport(ctx, "stale")
	if stale.Status != entity.AIReportFailed || !strings.Contains(stale.Error, "interrupted") {
		t.Errorf("stale report: %s %q", stale.Status, stale.Error)
	}
	if model.calls() != 1 {
		t.Errorf("model calls = %d, want the released slot run again by the first tick", model.calls())
	}
}

func TestRunReturnsItsOwnCopy(t *testing.T) {
	s, _, _, _ := newTestService(t, mustUTC(t, "2026-11-04T10:00:00Z"))
	report, err := s.Run(context.Background(), "admin", &entity.AIReportRunRequest{Language: "es"})
	if err != nil {
		t.Fatal(err)
	}
	// read while the run fills its report; -race fails if they share it
	_, _ = json.Marshal(report)
	wait(s)
	if report.Status != entity.AIReportRunning || len(report.Analyses) != 0 {
		t.Errorf("returned report changed after the call: %s", report.Status)
	}
}

func TestRecipientsDeduplicateAddresses(t *testing.T) {
	s, db, _, _ := newTestService(t, mustUTC(t, "2026-11-04T10:00:00Z"))
	subscribe(t, db, "Ops@Example.com", "es", true)
	subscribe(t, db, " ops@example.com", "en", true)
	got, err := s.recipients(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("recipients = %+v, want one per address", got)
	}
}

func TestStopRefusesNewRuns(t *testing.T) {
	s, _, _, _ := newTestService(t, mustUTC(t, "2026-11-04T10:00:00Z"))
	s.Stop()
	if _, err := s.Run(context.Background(), "admin", &entity.AIReportRunRequest{DryRun: true}); !errors.Is(err, ErrStopped) {
		t.Errorf("err = %v, want ErrStopped", err)
	}
}
