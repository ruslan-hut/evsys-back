package mcpserver

import (
	"fmt"
	"math"
	"strings"
	"time"
)

var timeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
}

const dateLayout = "2006-01-02"

// parseTime reads an RFC 3339 timestamp, a timestamp without zone (taken as
// UTC) or a bare date. A bare date as the end of a period means the end of
// that day.
func parseTime(value string, endOfDay bool) (time.Time, error) {
	value = strings.TrimSpace(value)
	if t, err := time.Parse(dateLayout, value); err == nil {
		if endOfDay {
			return t.Add(24*time.Hour - time.Millisecond), nil
		}
		return t, nil
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot read %q as a time: use RFC 3339 (2026-10-01T08:00:00Z) or a date (2026-10-01)", value)
}

// period resolves optional from/to inputs: to defaults to now, from to
// defaultSpan before to.
func period(from, to string, now time.Time, defaultSpan time.Duration) (time.Time, time.Time, error) {
	end := now.UTC()
	if to != "" {
		t, err := parseTime(to, true)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("to: %w", err)
		}
		end = t
	}
	start := end.Add(-defaultSpan)
	if from != "" {
		t, err := parseTime(from, false)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("from: %w", err)
		}
		start = t
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("'to' (%s) must be after 'from' (%s)", end.Format(time.RFC3339), start.Format(time.RFC3339))
	}
	return start, end, nil
}

// optionalTime parses an optional bound; nil when absent.
func optionalTime(value string, endOfDay bool) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	t, err := parseTime(value, endOfDay)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// limit applies a default and an upper bound to a requested count.
func limit(requested, defaultValue, maxValue int) int {
	if requested <= 0 {
		return defaultValue
	}
	if requested > maxValue {
		return maxValue
	}
	return requested
}

// timestamp renders a time for output, leaving unset times out.
func timestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// minutes renders a duration in minutes, to a tenth.
func minutes(d time.Duration) float64 {
	return math.Round(d.Minutes()*10) / 10
}
