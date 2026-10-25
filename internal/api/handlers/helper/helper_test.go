package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseLogFilter(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		from    string
		to      string
		charger string
		limit   int64
	}{
		{
			name:  "empty query",
			query: "",
		},
		{
			name:  "dates only",
			query: "from=2026-07-01&to=2026-07-24",
			from:  "2026-07-01T00:00:00Z",
			to:    "2026-07-24T23:59:59Z",
		},
		{
			name:  "full datetime keeps its time",
			query: "from=2026-07-01T08:30:00&to=2026-07-01T09:00:00",
			from:  "2026-07-01T08:30:00Z",
			to:    "2026-07-01T09:00:00Z",
		},
		{
			name:  "rfc3339 with offset converts to utc",
			query: "from=2026-07-01T10:00:00%2B02:00",
			from:  "2026-07-01T08:00:00Z",
		},
		{
			name:  "unparsable date is ignored",
			query: "from=yesterday",
		},
		{
			name:    "charger and limit",
			query:   "charge_point_id=CP001&limit=500",
			charger: "CP001",
			limit:   500,
		},
		{
			name:  "invalid limit is ignored",
			query: "limit=none",
		},
		{
			name:  "negative limit is ignored",
			query: "limit=-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/v1/log/sys?"+tt.query, nil)
			filter := parseLogFilter(r)

			if got := formatFilterTime(filter.From); got != tt.from {
				t.Errorf("from: got %q, want %q", got, tt.from)
			}
			if got := formatFilterTime(filter.To); got != tt.to {
				t.Errorf("to: got %q, want %q", got, tt.to)
			}
			if filter.ChargePointId != tt.charger {
				t.Errorf("charge point: got %q, want %q", filter.ChargePointId, tt.charger)
			}
			if filter.Limit != tt.limit {
				t.Errorf("limit: got %d, want %d", filter.Limit, tt.limit)
			}
		})
	}
}

func formatFilterTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func TestOptionsAllowsTheAdminMethods(t *testing.T) {
	handler := Options()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a preflight must not reach the handler")
	}))
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/mail/subscriptions/x", nil)
	req.Header.Set("Origin", "https://app.wattbrews.me")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	allowed := rec.Header().Get("Access-Control-Allow-Methods")
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		if !strings.Contains(allowed, method) {
			t.Errorf("preflight allows %q, missing %s", allowed, method)
		}
	}
}
