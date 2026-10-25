package http

import (
	"encoding/json"
	"evsys-back/config"
	"evsys-back/entity"
	"evsys-back/impl/aireport"
	"evsys-back/impl/authenticator"
	"evsys-back/impl/core"
	databasemock "evsys-back/impl/database-mock"
	"evsys-back/impl/openai"
	"evsys-back/impl/reports"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const operatorApiToken = "opr00000000000000000000000000001"

func newAIReportTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := databasemock.NewMockDB()
	db.SeedUser(&entity.User{Username: "boss", Role: "admin", AccessLevel: 10, Token: adminApiToken})
	db.SeedUser(&entity.User{Username: "ops", Role: "operator", AccessLevel: 10, Token: operatorApiToken})
	db.SeedUser(&entity.User{Username: "driver", Role: "user", AccessLevel: 1, Token: userApiToken})

	c := core.New(log, db)
	c.SetAuth(authenticator.New(log, db))
	c.SetReports(reports.New(db, log))
	c.SetAIReportService(aireport.New(db, openai.New(openai.Config{}, log), nil, log))

	srv := httptest.NewServer(NewServer(&config.Config{}, log, c, nil).httpServer.Handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestAIReportRoutes(t *testing.T) {
	srv := newAIReportTestServer(t)

	tests := []struct {
		name   string
		method string
		path   string
		token  string
		body   string
		status int
	}{
		{"admin reads settings", http.MethodGet, "/api/v1/ai/settings", adminApiToken, "", http.StatusOK},
		{"operator may not", http.MethodGet, "/api/v1/ai/settings", operatorApiToken, "", http.StatusForbidden},
		{"user may not", http.MethodGet, "/api/v1/ai/settings", userApiToken, "", http.StatusForbidden},
		{"anonymous may not", http.MethodGet, "/api/v1/ai/settings", "", "", http.StatusUnauthorized},
		{"admin lists reports", http.MethodGet, "/api/v1/ai/reports", adminApiToken, "", http.StatusOK},
		{"admin reads usage", http.MethodGet, "/api/v1/ai/usage?from=2026-10-01&to=2026-10-31", adminApiToken, "", http.StatusOK},
		{"run without a key is refused", http.MethodPost, "/api/v1/ai/reports/run", adminApiToken, `{}`, http.StatusBadRequest},
		{"dry run works without a key", http.MethodPost, "/api/v1/ai/reports/run", adminApiToken, `{"dry_run":true}`, http.StatusAccepted},
		{"invalid language", http.MethodPost, "/api/v1/ai/reports/run", adminApiToken, `{"language":"fr"}`, http.StatusBadRequest},
		{"unknown report", http.MethodGet, "/api/v1/ai/reports/nope", adminApiToken, "", http.StatusNotFound},
		// the routes of the /api/v1 mount still answer next to the ai ones
		{"mail subscriptions still routed", http.MethodGet, "/api/v1/mail/subscriptions", adminApiToken, "", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := do(t, tt.method, srv.URL+tt.path, tt.token, "application/json", tt.body)
			assert.Equal(t, tt.status, resp.StatusCode, string(body))
		})
	}
}

func TestAIReportSettingsRoundTrip(t *testing.T) {
	srv := newAIReportTestServer(t)

	resp, body := do(t, http.MethodPut, srv.URL+"/api/v1/ai/settings", adminApiToken, "application/json",
		`{"hour": 8, "monthly_budget_usd": 5, "model": "gpt-6-luna"}`)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	resp, body = do(t, http.MethodGet, srv.URL+"/api/v1/ai/settings", adminApiToken, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	var view map[string]any
	require.NoError(t, json.Unmarshal(body, &view))
	assert.Equal(t, float64(8), view["hour"])
	assert.Equal(t, float64(5), view["monthly_budget_usd"])
	assert.Equal(t, "gpt-6-luna", view["model"])
	assert.Equal(t, "Europe/Madrid", view["time_zone"])
	assert.Equal(t, false, view["has_api_key"])
	assert.NotEmpty(t, view["default_system_prompt"])
	assert.NotContains(t, string(body), "api_key\":\"", "the key never leaves the server")
}

// The settings form sends every field; the model's default effort is "".
func TestAIReportSettingsReasoningEffort(t *testing.T) {
	srv := newAIReportTestServer(t)
	for _, tt := range []struct {
		body   string
		status int
	}{
		{`{"reasoning_effort": ""}`, http.StatusOK},
		{`{"reasoning_effort": "low"}`, http.StatusOK},
		{`{"reasoning_effort": "turbo"}`, http.StatusBadRequest},
		{`{"enabled": true, "model": "gpt-6.1-sol", "reasoning_effort": "", "max_output_tokens": 8000, "weekday": 1, "hour": 7, "time_zone": "Europe/Madrid", "period_days": 7, "monthly_budget_usd": 0, "system_prompt": ""}`, http.StatusOK},
	} {
		resp, body := do(t, http.MethodPut, srv.URL+"/api/v1/ai/settings", adminApiToken, "application/json", tt.body)
		assert.Equal(t, tt.status, resp.StatusCode, "%s: %s", tt.body, string(body))
	}
}

func TestMailSubscriptionAIAnalysis(t *testing.T) {
	srv := newAIReportTestServer(t)

	resp, body := do(t, http.MethodPost, srv.URL+"/api/v1/mail/subscriptions", adminApiToken, "application/json",
		`{"email":"ops@example.com","report":"ai_analysis","enabled":true}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(body))
	var sub entity.MailSubscription
	require.NoError(t, json.Unmarshal(body, &sub))
	assert.Equal(t, entity.MailLanguageES, sub.Language, "spanish by default")
	assert.Equal(t, "", sub.Period, "no period: the statistic scheduler must never see it")

	// an update from a client unaware of the new fields keeps them
	resp, body = do(t, http.MethodPut, srv.URL+"/api/v1/mail/subscriptions/"+sub.Id, adminApiToken, "application/json",
		`{"email":"ops@example.com","period":"weekly","user_group":"","enabled":false}`)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	require.NoError(t, json.Unmarshal(body, &sub))
	assert.Equal(t, entity.MailReportAIAnalysis, sub.Report)
	assert.Equal(t, "", sub.UserGroup)

	resp, body = do(t, http.MethodGet, srv.URL+"/api/v1/mail/subscriptions", adminApiToken, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	var list []entity.MailSubscription
	require.NoError(t, json.Unmarshal(body, &list))
	require.Len(t, list, 1)
	assert.Equal(t, entity.MailReportAIAnalysis, list[0].Report)
	assert.False(t, list[0].Enabled)

	// a statistic subscription still needs its period and group
	resp, body = do(t, http.MethodPost, srv.URL+"/api/v1/mail/subscriptions", adminApiToken, "application/json",
		`{"email":"ops@example.com","enabled":true}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, string(body))
}
