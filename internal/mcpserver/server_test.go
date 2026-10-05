package mcpserver

import (
	"context"
	"encoding/json"
	"evsys-back/entity"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

// fakeCore returns canned data and records the arguments it was called with.
type fakeCore struct {
	chargePoints []*entity.ChargePoint
	active       []*entity.ChargeState
	transactions []*entity.Transaction
	detail       *entity.ChargeState
	logRecords   any
	panicOnUsers bool

	lastFilter    *entity.TransactionFilter
	lastLogName   string
	lastLogFilter *entity.LogFilter
	lastFrom      time.Time
	lastTo        time.Time
	lastGroup     string
	lastGroupBy   string
	lastMinSess   int
	lastMaxSegs   int
}

func (f *fakeCore) InspectLocations(_ context.Context, _ *entity.User) ([]*entity.Location, error) {
	return []*entity.Location{{Id: "LOC1", Name: "Office", PowerLimit: 63, ChargePoints: f.chargePoints}}, nil
}
func (f *fakeCore) InspectChargePoints(_ context.Context, _ *entity.User, _ string) ([]*entity.ChargePoint, error) {
	return f.chargePoints, nil
}
func (f *fakeCore) InspectChargePoint(_ context.Context, _ *entity.User, id string) (*entity.ChargePoint, error) {
	for _, cp := range f.chargePoints {
		if cp.Id == id {
			return cp, nil
		}
	}
	return nil, fmt.Errorf("charge point %s %w", id, entity.ErrNotFound)
}
func (f *fakeCore) InspectActiveTransactions(_ context.Context, _ *entity.User) ([]*entity.ChargeState, error) {
	return f.active, nil
}
func (f *fakeCore) InspectTransactions(_ context.Context, _ *entity.User, filter *entity.TransactionFilter) ([]*entity.Transaction, error) {
	f.lastFilter = filter
	if filter.Limit > 0 && int64(len(f.transactions)) > filter.Limit {
		return f.transactions[:filter.Limit], nil
	}
	return f.transactions, nil
}
func (f *fakeCore) InspectTransaction(_ context.Context, _ *entity.User, id int) (*entity.ChargeState, error) {
	if f.detail == nil || f.detail.TransactionId != id {
		return nil, fmt.Errorf("transaction %d %w", id, entity.ErrNotFound)
	}
	return f.detail, nil
}
func (f *fakeCore) InspectLog(_ context.Context, _ *entity.User, name string, filter *entity.LogFilter) (any, error) {
	f.lastLogName, f.lastLogFilter = name, filter
	return f.logRecords, nil
}
func (f *fakeCore) InspectErrorSummary(_ context.Context, _ *entity.User, from, to time.Time, _ string) ([]*entity.ErrorSummary, error) {
	f.lastFrom, f.lastTo = from, to
	return []*entity.ErrorSummary{{ChargePointId: "CP1", ErrorCode: "GroundFailure", Count: 3}, {ChargePointId: "CP2", ErrorCode: "OverVoltage", Count: 2}}, nil
}
func (f *fakeCore) stats(from, to time.Time, group, by string) ([]any, error) {
	f.lastFrom, f.lastTo, f.lastGroup, f.lastGroupBy = from, to, group, by
	return []any{map[string]any{"total": 1000, "count": 2}}, nil
}
func (f *fakeCore) MonthlyStats(_ context.Context, _ *entity.User, from, to time.Time, g string) ([]any, error) {
	return f.stats(from, to, g, "month")
}
func (f *fakeCore) UsersStats(_ context.Context, _ *entity.User, from, to time.Time, g string) ([]any, error) {
	return f.stats(from, to, g, "user")
}
func (f *fakeCore) ChargerStats(_ context.Context, _ *entity.User, from, to time.Time, g string) ([]any, error) {
	return f.stats(from, to, g, "charger")
}
func (f *fakeCore) ExportStats(_ context.Context, _ *entity.User, from, to time.Time, g string) ([]any, error) {
	return f.stats(from, to, g, "hour")
}
func (f *fakeCore) PowerStatsReport(_ context.Context, _ *entity.User, from, to time.Time, _, g, by string) ([]*entity.PowerStats, error) {
	f.lastFrom, f.lastTo, f.lastGroup, f.lastGroupBy = from, to, g, by
	return []*entity.PowerStats{{ChargePointId: "CP1", Sessions: 2, MaxPower: 7200}}, nil
}
func (f *fakeCore) StationUptimeReport(_ context.Context, _ *entity.User, _, _ time.Time, _ string) ([]*entity.StationUptime, error) {
	return []*entity.StationUptime{{ChargePointId: "CP1", OnlineDuration: time.Hour, UptimePercent: 50, FinalState: entity.StateOnline}}, nil
}
func (f *fakeCore) StationStatusReport(_ context.Context, _ *entity.User, _ string) ([]*entity.StationStatus, error) {
	return []*entity.StationStatus{
		{ChargePointId: "CP1", State: entity.StateOnline, Since: testNow.Add(-48 * time.Hour)},
		{ChargePointId: "CP2", State: entity.StateOffline, Since: testNow.Add(-72 * time.Hour), Duration: 72 * time.Hour},
	}, nil
}
func (f *fakeCore) SiteConcurrencyReport(_ context.Context, _ *entity.User, from, to time.Time, _ string, minSessions, maxSegments int) ([]*entity.SiteConcurrency, error) {
	f.lastFrom, f.lastTo, f.lastMinSess, f.lastMaxSegs = from, to, minSessions, maxSegments
	return nil, nil
}
func (f *fakeCore) GetUsers(_ context.Context, user *entity.User) ([]*entity.User, error) {
	if f.panicOnUsers {
		var nilUser *entity.User
		_ = nilUser.Username
	}
	if !user.IsAdmin() {
		return nil, fmt.Errorf("access denied")
	}
	return []*entity.User{
		{Username: "zed", Name: "Zed", Role: "user", Token: "12345678901234567890123456789000", Password: "$2a$hash"},
		{Username: "ann", Name: "Ann", Email: "ann@example.com", Role: "operator"},
	}, nil
}
func (f *fakeCore) ListUserTags(_ context.Context, _ *entity.User) ([]*entity.UserTag, error) {
	return []*entity.UserTag{{IdTag: "B2", Username: "zed", Note: "blue card"}, {IdTag: "A1", Username: "ann"}}, nil
}
func (f *fakeCore) GetPaymentRetryQueue(_ context.Context, _ *entity.User) ([]*entity.PaymentRetryView, error) {
	return nil, nil
}
func (f *fakeCore) GetWebhookHealth(_ context.Context, _ *entity.User) ([]*entity.WebhookHealthView, error) {
	return []*entity.WebhookHealthView{{Name: "crm", Pending: 1}}, nil
}
func (f *fakeCore) ListWebhookFailures(_ context.Context, _ *entity.User) ([]*entity.WebhookDeliveryView, error) {
	return nil, nil
}

type fakeVerifier map[string]*entity.User

func (v fakeVerifier) VerifyAccessToken(_ context.Context, token string) (*entity.User, *entity.OAuthToken, error) {
	user, ok := v[token]
	if !ok {
		return nil, nil, entity.ErrInvalidToken
	}
	return user, &entity.OAuthToken{Scope: entity.OAuthScope, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

var (
	admin    = &entity.User{Username: "boss", Role: "admin", AccessLevel: 10}
	operator = &entity.User{Username: "op", Role: "operator", AccessLevel: 5}
)

const resourceMetadata = "https://ev.example.com/api/v1/oauth/protected-resource"

func newTestServer(t *testing.T, core *fakeCore) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(log, core, fakeVerifier{"admin-token": admin, "operator-token": operator}, resourceMetadata)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

type bearer struct {
	token string
	base  http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r)
}

func connect(t *testing.T, url, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   url,
		HTTPClient: &http.Client{Transport: bearer{token: token, base: http.DefaultTransport}},
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// call invokes a tool and decodes its JSON text result. It fails the test on a
// tool error unless wantErr is set, in which case it returns the error text.
func call(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (map[string]any, string) {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	require.Len(t, res.Content, 1)
	text := res.Content[0].(*mcp.TextContent).Text
	if res.IsError {
		return nil, text
	}
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &out), text)
	return out, ""
}

func sampleData() *fakeCore {
	yes := true
	return &fakeCore{
		chargePoints: []*entity.ChargePoint{
			{
				Id: "CP1", Title: "Gate", IsEnabled: true, IsOnline: true, Status: "Available", Model: "Wallbox", TriggerMessage: &yes,
				Connectors: []*entity.Connector{
					{Id: 1, Status: "Charging", TransactionId: 77, CurrentPowerLimit: 16},
					{Id: 2, Status: "Faulted", ErrorCode: "GroundFailure", StatusTime: testNow.Add(-time.Hour)},
				},
			},
			{Id: "CP2", IsEnabled: true, IsOnline: false, EventTime: testNow.Add(-2 * time.Hour), Connectors: []*entity.Connector{{Id: 1, Status: "Available"}}},
			{Id: "CP3", IsEnabled: false, Connectors: []*entity.Connector{{Id: 1, Status: "Faulted"}}},
		},
		active: []*entity.ChargeState{{
			TransactionId: 77, ChargePointId: "CP1", ConnectorId: 1, Status: "Charging", IdTag: "B2",
			TimeStarted: testNow.Add(-30 * time.Minute), Consumed: 3500, PowerRate: 7100, PowerLimit: 16,
			MeterValues: []entity.TransactionMeter{{Time: testNow.Add(-time.Minute), PowerRate: 7100, Voltage: 230, CurrentImport: 15.8, CurrentOffered: 16}},
		}},
	}
}

func TestUnauthenticated(t *testing.T) {
	srv := newTestServer(t, sampleData())
	for _, token := range []string{"", "wrong"} {
		req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, `Bearer resource_metadata="`+resourceMetadata+`"`, resp.Header.Get("WWW-Authenticate"))
	}
}

func TestToolsAreReadOnly(t *testing.T) {
	srv := newTestServer(t, sampleData())
	session := connect(t, srv.URL, "admin-token")
	assert.Contains(t, session.InitializeResult().Instructions, "system_overview")

	res, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
		require.NotNil(t, tool.Annotations, tool.Name)
		assert.True(t, tool.Annotations.ReadOnlyHint, tool.Name)
		assert.NotEmpty(t, tool.Description, tool.Name)
	}
	for _, want := range []string{
		"system_overview", "list_charge_points", "get_charge_point", "list_locations",
		"list_active_transactions", "search_transactions", "get_transaction",
		"read_log", "error_summary", "energy_report", "power_report",
		"station_uptime", "station_status", "site_concurrency",
		"list_users", "list_user_tags", "payment_retry_queue", "webhook_status",
	} {
		assert.True(t, names[want], "missing tool %s", want)
	}
}

func TestSystemOverview(t *testing.T) {
	srv := newTestServer(t, sampleData())
	session := connect(t, srv.URL, "operator-token")
	out, _ := call(t, session, "system_overview", nil)

	cps := out["charge_points"].(map[string]any)
	assert.EqualValues(t, 3, cps["total"])
	assert.EqualValues(t, 2, cps["enabled"])
	assert.EqualValues(t, 1, cps["online"])
	assert.EqualValues(t, 1, cps["offline_enabled"])
	assert.EqualValues(t, 1, cps["disabled"])

	byStatus := out["connectors"].(map[string]any)["by_status"].(map[string]any)
	assert.EqualValues(t, 1, byStatus["Faulted"], "the disabled CP3 must not count")

	problems := out["problems"].([]any)
	require.Len(t, problems, 2)
	kinds := []string{problems[0].(map[string]any)["problem"].(string), problems[1].(map[string]any)["problem"].(string)}
	assert.ElementsMatch(t, []string{"connector error", "offline"}, kinds)
	for _, p := range problems {
		p := p.(map[string]any)
		if p["problem"] == "offline" {
			// since is the disconnect from the sys log, not the last message
			assert.Equal(t, "2026-10-02T12:00:00Z", p["since"])
			assert.Equal(t, "2026-10-05T10:00:00Z", p["last_event"])
		}
	}

	active := out["active_sessions"].(map[string]any)
	assert.EqualValues(t, 1, active["count"])
	assert.EqualValues(t, 7100, active["total_power_w"])
}

func TestChargePoints(t *testing.T) {
	srv := newTestServer(t, sampleData())
	session := connect(t, srv.URL, "admin-token")

	out, _ := call(t, session, "list_charge_points", map[string]any{"problems_only": true})
	assert.EqualValues(t, 2, out["count"])

	out, _ = call(t, session, "get_charge_point", map[string]any{"charge_point_id": "CP1"})
	cp := out["charge_point"].(map[string]any)
	assert.Equal(t, "Wallbox", cp["model"])
	conns := cp["connectors"].([]any)
	assert.EqualValues(t, 16, conns[0].(map[string]any)["current_limit_a"])
	assert.NotContains(t, cp, "last_event", "unset times are left out")

	_, errText := call(t, session, "get_charge_point", map[string]any{"charge_point_id": "nope"})
	assert.Contains(t, errText, "not found")
}

func TestSearchTransactions(t *testing.T) {
	core := sampleData()
	for i := 0; i < 5; i++ {
		start := testNow.Add(-time.Duration(i+1) * 24 * time.Hour)
		core.transactions = append(core.transactions, &entity.Transaction{
			TransactionId: 100 + i, ChargePointId: "CP1", IdTag: "B2", IsFinished: true,
			TimeStart: start, TimeStop: start.Add(2 * time.Hour), MeterStart: 1000, MeterStop: 15000,
			PaymentAmount: 450, UserTag: &entity.UserTag{Username: "zed", Note: "blue card"},
		})
	}
	srv := newTestServer(t, core)
	session := connect(t, srv.URL, "admin-token")

	out, _ := call(t, session, "search_transactions", map[string]any{
		"from": "2026-09-01", "to": "2026-09-30", "charge_point_id": " CP1 ", "limit": 3,
	})
	f := core.lastFilter
	assert.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), *f.From)
	assert.Equal(t, time.Date(2026, 9, 30, 23, 59, 59, int(999*time.Millisecond), time.UTC), *f.To)
	assert.Equal(t, "CP1", f.ChargePointId)
	assert.EqualValues(t, 3, f.Limit)
	assert.True(t, f.SkipMeterValues)

	assert.EqualValues(t, 3, out["count"])
	assert.Contains(t, out, "truncated")
	totals := out["totals"].(map[string]any)
	assert.EqualValues(t, 3*14000, totals["energy_wh"])
	assert.EqualValues(t, 6, totals["duration_h"])
	row := out["transactions"].([]any)[0].(map[string]any)
	assert.EqualValues(t, 7000, row["avg_power_w"])
	assert.Equal(t, "zed", row["username"])
	assert.Equal(t, "blue card", row["id_tag_note"])

	_, errText := call(t, session, "search_transactions", map[string]any{"from": "2026-10-02", "to": "2026-10-01"})
	assert.Contains(t, errText, "must be after")
	_, errText = call(t, session, "search_transactions", map[string]any{"from": "yesterday"})
	assert.Contains(t, errText, "cannot read")
}

func TestGetTransactionRedactsAndSamples(t *testing.T) {
	core := sampleData()
	meters := make([]entity.TransactionMeter, 500)
	for i := range meters {
		meters[i] = entity.TransactionMeter{Time: testNow.Add(time.Duration(i) * time.Minute), ConsumedEnergy: i * 10}
	}
	core.detail = &entity.ChargeState{
		TransactionId: 9, IsFinished: true, TimeStarted: testNow, TimeStop: testNow.Add(500 * time.Minute),
		CanStop: true, Duration: 30000,
		MeterValues:   meters,
		PaymentMethod: &entity.PaymentMethod{CardNumber: "**** 4242", Identifier: "card-token-secret", CofTid: "cof-secret"},
		PaymentOrders: []entity.PaymentOrder{{Order: 5, Amount: 450, Identifier: "card-token-secret"}},
	}
	srv := newTestServer(t, core)
	session := connect(t, srv.URL, "admin-token")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_transaction", Arguments: map[string]any{"transaction_id": 9, "max_meter_points": 50}})
	require.NoError(t, err)
	text := res.Content[0].(*mcp.TextContent).Text
	assert.NotContains(t, text, "card-token-secret")
	assert.NotContains(t, text, "cof-secret")
	assert.NotContains(t, text, "can_stop")
	assert.Contains(t, text, "4242")

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &out))
	assert.EqualValues(t, 500, out["meter_value_count"])
	values := out["meter_values"].([]any)
	require.Len(t, values, 50)
	assert.EqualValues(t, 0, values[0].(map[string]any)["consumed_wh"])
	assert.EqualValues(t, 4990, values[49].(map[string]any)["consumed_wh"], "the last reading is kept")
	assert.EqualValues(t, 500, out["duration_min"])
}

func TestReadLog(t *testing.T) {
	core := sampleData()
	core.logRecords = []entity.FeatureMessage{{Feature: "StatusNotification", ChargePointId: "CP1", Text: "Faulted"}}
	srv := newTestServer(t, core)
	session := connect(t, srv.URL, "admin-token")

	out, _ := call(t, session, "read_log", map[string]any{
		"log": "SYS", "search": "fault", "category": "StatusNotification", "limit": 5000, "from": "2026-10-01",
	})
	assert.Equal(t, "sys", core.lastLogName)
	assert.Equal(t, "fault", core.lastLogFilter.Search)
	assert.Equal(t, "StatusNotification", core.lastLogFilter.Category)
	assert.EqualValues(t, maxLogLimit, core.lastLogFilter.Limit)
	assert.Nil(t, core.lastLogFilter.To)
	assert.EqualValues(t, 1, out["count"])

	_, errText := call(t, session, "read_log", map[string]any{"log": "audit"})
	assert.Contains(t, errText, "unknown log")
}

func TestReports(t *testing.T) {
	core := sampleData()
	srv := newTestServer(t, core)
	session := connect(t, srv.URL, "admin-token")

	out, _ := call(t, session, "energy_report", map[string]any{"group_by": "charger", "from": "2026-09-01", "to": "2026-09-30"})
	assert.Equal(t, "charger", core.lastGroupBy)
	assert.Equal(t, "default", core.lastGroup)
	assert.Contains(t, out["note"], "'user' field")
	_, errText := call(t, session, "energy_report", map[string]any{"group_by": "week"})
	assert.Contains(t, errText, "expected month")

	call(t, session, "power_report", map[string]any{"group_by": "Hour", "to": "2026-09-30"})
	assert.Equal(t, "hour", core.lastGroupBy)
	assert.Equal(t, time.Date(2026, 9, 30, 23, 59, 59, int(999*time.Millisecond), time.UTC), core.lastTo)
	assert.Equal(t, 7*24*time.Hour, core.lastTo.Sub(core.lastFrom), "from defaults to a week before to")

	call(t, session, "site_concurrency", map[string]any{"max_segments": 99999})
	assert.Equal(t, 2, core.lastMinSess)
	assert.Equal(t, maxConcurrencySegments, core.lastMaxSegs)

	out, _ = call(t, session, "station_uptime", nil)
	assert.EqualValues(t, 3600, out["stations"].([]any)[0].(map[string]any)["online_seconds"])

	out, _ = call(t, session, "error_summary", nil)
	assert.EqualValues(t, 5, out["total_errors"])
}

func TestUsersAreSanitized(t *testing.T) {
	srv := newTestServer(t, sampleData())

	admin := connect(t, srv.URL, "admin-token")
	res, err := admin.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_users", Arguments: map[string]any{}})
	require.NoError(t, err)
	text := res.Content[0].(*mcp.TextContent).Text
	assert.NotContains(t, text, "12345678901234567890123456789000")
	assert.NotContains(t, text, "$2a$")
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &out))
	users := out["users"].([]any)
	assert.Equal(t, "ann", users[0].(map[string]any)["username"], "sorted by username")

	out, _ = call(t, admin, "list_users", map[string]any{"search": "EXAMPLE.com"})
	assert.EqualValues(t, 1, out["count"])

	out, _ = call(t, admin, "list_user_tags", map[string]any{"search": "blue"})
	assert.EqualValues(t, 1, out["count"])

	// operators see what the REST API shows them, which excludes the user list
	op := connect(t, srv.URL, "operator-token")
	_, errText := call(t, op, "list_users", nil)
	assert.Contains(t, errText, "access denied")
}

func TestResultSizeIsBounded(t *testing.T) {
	core := sampleData()
	big := make([]entity.FeatureMessage, 1000)
	for i := range big {
		big[i] = entity.FeatureMessage{Text: strings.Repeat("x", 500)}
	}
	core.logRecords = big
	srv := newTestServer(t, core)
	session := connect(t, srv.URL, "admin-token")
	_, errText := call(t, session, "read_log", map[string]any{"log": "sys"})
	assert.Contains(t, errText, "result too large")
}

func TestToolPanicIsContained(t *testing.T) {
	core := sampleData()
	core.panicOnUsers = true
	srv := newTestServer(t, core)
	session := connect(t, srv.URL, "admin-token")
	_, errText := call(t, session, "list_users", nil)
	assert.Contains(t, errText, "internal error")
	// the server is still serving
	out, _ := call(t, session, "station_status", nil)
	assert.NotNil(t, out)
}
