package aireport

import (
	"testing"
	"time"
)

func mustUTC(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestLastSlot(t *testing.T) {
	madrid, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		now  string
		want string
	}{
		// winter time (CET, UTC+1): 07:00 Madrid is 06:00 UTC
		{"monday after the slot", "2026-11-02T06:30:00Z", "2026-11-02T06:00:00Z"},
		{"monday exactly at the slot", "2026-11-02T06:00:00Z", "2026-11-02T06:00:00Z"},
		{"monday before the slot", "2026-11-02T05:59:00Z", "2026-10-26T06:00:00Z"},
		{"sunday", "2026-11-01T12:00:00Z", "2026-10-26T06:00:00Z"},
		// summer time (CEST, UTC+2): 07:00 Madrid is 05:00 UTC
		{"summer monday", "2026-10-19T05:10:00Z", "2026-10-19T05:00:00Z"},
		// the clocks went back on Sunday 2026-10-25; the next slot is in CET
		{"first monday after the change", "2026-10-26T06:00:00Z", "2026-10-26T06:00:00Z"},
		{"one hour earlier is still last week's slot", "2026-10-26T05:30:00Z", "2026-10-19T05:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastSlot(mustUTC(t, tt.now), time.Monday, 7, madrid)
			if !got.Equal(mustUTC(t, tt.want)) {
				t.Errorf("lastSlot(%s) = %s, want %s", tt.now, got.UTC().Format(time.RFC3339), tt.want)
			}
		})
	}
}

func TestNextSlot(t *testing.T) {
	madrid, _ := time.LoadLocation("Europe/Madrid")
	tests := []struct {
		name string
		now  string
		want string
	}{
		{"saturday before the change", "2026-10-24T10:00:00Z", "2026-10-26T06:00:00Z"},
		{"exactly at the slot gives the next week", "2026-10-26T06:00:00Z", "2026-11-02T06:00:00Z"},
		{"monday before the slot", "2026-10-26T05:00:00Z", "2026-10-26T06:00:00Z"},
		// spring forward on Sunday 2026-03-29
		{"first monday of summer time", "2026-03-28T12:00:00Z", "2026-03-30T05:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextSlot(mustUTC(t, tt.now), time.Monday, 7, madrid)
			if !got.Equal(mustUTC(t, tt.want)) {
				t.Errorf("nextSlot(%s) = %s, want %s", tt.now, got.UTC().Format(time.RFC3339), tt.want)
			}
		})
	}
}

func TestPeriodRange(t *testing.T) {
	madrid, _ := time.LoadLocation("Europe/Madrid")
	tests := []struct {
		name     string
		at       string
		days     int
		from, to string
	}{
		// the week crossing the change has 7 days but 169 hours
		{"week with the clock change", "2026-10-26T06:00:00Z", 7, "2026-10-18T22:00:00Z", "2026-10-25T23:00:00Z"},
		{"plain winter week", "2026-11-02T06:00:00Z", 7, "2026-10-25T23:00:00Z", "2026-11-01T23:00:00Z"},
		{"one day", "2026-11-02T06:00:00Z", 1, "2026-10-31T23:00:00Z", "2026-11-01T23:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to := periodRange(mustUTC(t, tt.at), tt.days, madrid)
			if !from.Equal(mustUTC(t, tt.from)) || !to.Equal(mustUTC(t, tt.to)) {
				t.Errorf("periodRange = [%s, %s), want [%s, %s)",
					from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339), tt.from, tt.to)
			}
		})
	}
}
