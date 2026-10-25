package aireport

import (
	"context"
	"encoding/json"
	"evsys-back/entity"
	databasemock "evsys-back/impl/database-mock"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// networkRepo is the mock database with a network in it: two locations,
// their charge points, a week of sys_log and sessions.
type networkRepo struct {
	*databasemock.MockDB
	at time.Time
}

func (r *networkRepo) GetAllLocations(context.Context) ([]*entity.Location, error) {
	return []*entity.Location{
		{Id: "ALP", Name: "Alpicat", ChargePoints: []*entity.ChargePoint{{Id: "Wallbox1"}, {Id: "Wallbox2"}}},
		{Id: "ELP", Name: "El Pedernoso", ChargePoints: []*entity.ChargePoint{{Id: "PE00004"}}},
	}, nil
}

func (r *networkRepo) GetChargePoints(context.Context, int, string) ([]*entity.ChargePoint, error) {
	return []*entity.ChargePoint{
		{Id: "Wallbox1", IsEnabled: true, LocationId: "ALP"},
		{Id: "Wallbox2", IsEnabled: true, IsOnline: true, LocationId: "ALP"},
		{Id: "PE00004", IsEnabled: true, IsOnline: true, LocationId: "ELP", Status: "Faulted", ErrorCode: "OtherError"},
	}, nil
}

func (r *networkRepo) ReadLog(_ context.Context, name string, _ *entity.LogFilter) (any, error) {
	if name != "sys" {
		return []entity.LogMessage{{Level: "error", Category: "pay", Text: "payment failed", Timestamp: r.at}}, nil
	}
	return []entity.FeatureMessage{
		{Feature: "WebSocket", ChargePointId: "Wallbox1", Text: "unregistered: total connections 9", Timestamp: r.at},
		{Feature: "StatusNotification", ChargePointId: "PE00004", Text: "connector #1: Faulted (OtherError; CO.30.003)", Timestamp: r.at},
		{Feature: "warning", ChargePointId: "*", Text: "transaction #5223 on Wallbox1 closed by system", Timestamp: r.at},
		{Feature: "warning", ChargePointId: "*", Text: "transaction #5224 on PE00004 closed by system", Timestamp: r.at},
	}, nil
}

func (r *networkRepo) GetFilteredTransactions(context.Context, *entity.TransactionFilter) ([]*entity.Transaction, error) {
	return []*entity.Transaction{
		{TransactionId: 1, ChargePointId: "Wallbox2", MeterStart: 0, MeterStop: 5000, TimeStart: r.at, TimeStop: r.at.Add(time.Hour)},
		{TransactionId: 2, ChargePointId: "PE00004", MeterStart: 0, MeterStop: 9000, TimeStart: r.at, TimeStop: r.at.Add(time.Hour)},
	}, nil
}

func (r *networkRepo) ErrorSummary(context.Context, time.Time, time.Time, string) ([]*entity.ErrorSummary, error) {
	return []*entity.ErrorSummary{
		{ChargePointId: "PE00004", ConnectorId: 1, ErrorCode: "OtherError", Count: 3},
		{ChargePointId: "Wallbox1", ConnectorId: 1, ErrorCode: "GroundFailure", Count: 1},
	}, nil
}

func newNetworkService(t *testing.T, now time.Time) (*Service, *networkRepo, *fakeModel, *fakeSender) {
	t.Helper()
	repo := &networkRepo{MockDB: databasemock.NewMockDB(), at: now.Add(-48 * time.Hour)}
	model := &fakeModel{configured: true}
	sender := &fakeSender{}
	s := New(repo, model, sender, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.now = func() time.Time { return now }
	return s, repo, model, sender
}

func TestCollectLimitedToLocations(t *testing.T) {
	now := mustUTC(t, "2026-11-02T06:20:00Z")
	s, _, _, _ := newNetworkService(t, now)
	ctx := context.Background()

	sc, err := s.resolveScope(ctx, []string{"ALP", "GONE", "ALP"})
	if err != nil {
		t.Fatal(err)
	}
	if sc.key != "ALP,GONE" {
		t.Errorf("scope key = %q", sc.key)
	}
	from, to := periodRange(now, 7, time.UTC)
	snap := s.collect(ctx, from, to, from.AddDate(0, 0, -7), "UTC", sc)
	data, _ := json.Marshal(snap)
	text := string(data)

	for _, other := range []string{"PE00004", "#5224", "payment failed"} {
		if strings.Contains(text, other) {
			t.Errorf("a report about Alpicat must not mention %q", other)
		}
	}
	for _, own := range []string{"Wallbox1", "Wallbox2", "#N on Wallbox1 closed by system", "Alpicat", "GroundFailure"} {
		if !strings.Contains(text, own) {
			t.Errorf("a report about Alpicat must mention %q", own)
		}
	}
	if snap.Network.Enabled != 2 || snap.Sessions.Finished != 1 || snap.Payments != nil || snap.Webhooks != nil || snap.BackendLog != nil {
		t.Errorf("network %+v, sessions %d, payments %v, webhooks %v", snap.Network, snap.Sessions.Finished, snap.Payments, snap.Webhooks)
	}
	if !strings.Contains(strings.Join(snap.Notes, "\n"), "GONE") {
		t.Errorf("a missing location must be noted: %v", snap.Notes)
	}

	// the whole network keeps everything
	all, _ := s.resolveScope(ctx, nil)
	whole := s.collect(ctx, from, to, from.AddDate(0, 0, -7), "UTC", all)
	if whole.Scope != nil || whole.Network.Enabled != 3 || whole.Sessions.Finished != 2 || whole.PaymentLog == nil {
		t.Errorf("whole network snapshot cut: %+v", whole.Network)
	}
}

func TestRunPerScopeAndLanguage(t *testing.T) {
	ctx := context.Background()
	s, repo, model, sender := newNetworkService(t, mustUTC(t, "2026-11-02T06:20:00Z"))
	for _, sub := range []*entity.MailSubscription{
		{Email: "boss@example.com", Language: "es"},
		{Email: "alp@example.com", Language: "es", LocationIds: []string{"ALP"}},
		{Email: "alp-en@example.com", Language: "en", LocationIds: []string{"ALP"}},
		{Email: "both@example.com", Language: "es", LocationIds: []string{"ELP", "ALP"}},
	} {
		sub.Report, sub.Enabled = entity.MailReportAIAnalysis, true
		sub.Normalize()
		if _, err := repo.SaveMailSubscription(ctx, sub); err != nil {
			t.Fatal(err)
		}
	}

	s.tick()

	// scopes: all (es), ALP (es, en), ALP+ELP (es)
	if got := model.calls(); got != 4 {
		t.Fatalf("model calls = %d, want one per scope and language (4)", got)
	}
	reports, _ := repo.ListAIReports(ctx, 1)
	full, _ := repo.GetAIReport(ctx, reports[0].Id)
	if len(full.Scopes) != 3 {
		t.Fatalf("scopes = %d, want 3", len(full.Scopes))
	}
	for _, d := range full.Deliveries {
		want := map[string]string{
			"boss@example.com": "", "alp@example.com": "ALP", "alp-en@example.com": "ALP", "both@example.com": "ALP,ELP",
		}[d.Email]
		if d.Scope != want || d.Error != "" {
			t.Errorf("delivery to %s: scope %q, error %q", d.Email, d.Scope, d.Error)
		}
	}
	for _, m := range sender.all() {
		switch m.to {
		case "alp@example.com", "alp-en@example.com":
			if !strings.Contains(m.subject, "Alpicat") || strings.Contains(m.subject, "Pedernoso") {
				t.Errorf("subject of %s: %q", m.to, m.subject)
			}
		case "boss@example.com":
			if strings.Contains(m.subject, "Alpicat") {
				t.Errorf("the whole-network subject names a location: %q", m.subject)
			}
		}
	}

	preview, err := s.Preview(ctx, full.Id, "ALP", "en")
	if err != nil || preview.Scope != "ALP" || !strings.Contains(preview.Html, "Alpicat") {
		t.Errorf("preview of ALP: %v %+v", err, preview)
	}
	if _, err = s.Preview(ctx, full.Id, "ELP", "es"); err == nil {
		t.Error("no analysis was made of ELP alone")
	}
}

func TestDeliverNeverCrossesScopes(t *testing.T) {
	report := &entity.AIReport{Analyses: []*entity.AIAnalysis{
		{Scope: "", Language: "es", Result: &entity.AIAnalysisResult{Status: "ok"}},
	}}
	if a := analysisFor(report, "ALP", "es"); a != nil {
		t.Error("the whole-network answer must not stand in for a location's")
	}
	report.Analyses = append(report.Analyses, &entity.AIAnalysis{Scope: "ALP", Language: "en", Result: &entity.AIAnalysisResult{Status: "ok"}})
	if a := analysisFor(report, "ALP", "es"); a == nil || a.Language != "en" {
		t.Error("another language of the same locations is a valid fallback")
	}
}
