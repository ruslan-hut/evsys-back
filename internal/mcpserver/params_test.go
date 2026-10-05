package mcpserver

import (
	"evsys-back/entity"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTime(t *testing.T) {
	cases := []struct {
		in       string
		endOfDay bool
		want     time.Time
	}{
		{"2026-10-01", false, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		{"2026-10-01", true, time.Date(2026, 10, 1, 23, 59, 59, int(999*time.Millisecond), time.UTC)},
		{"2026-10-01T08:30:00Z", true, time.Date(2026, 10, 1, 8, 30, 0, 0, time.UTC)},
		{"2026-10-01T10:30:00+02:00", false, time.Date(2026, 10, 1, 8, 30, 0, 0, time.UTC)},
		{"2026-10-01T08:30", false, time.Date(2026, 10, 1, 8, 30, 0, 0, time.UTC)},
		{" 2026-10-01 08:30:15 ", false, time.Date(2026, 10, 1, 8, 30, 15, 0, time.UTC)},
	}
	for _, c := range cases {
		got, err := parseTime(c.in, c.endOfDay)
		require.NoError(t, err, c.in)
		assert.True(t, c.want.Equal(got), "%q: got %s, want %s", c.in, got, c.want)
		assert.Equal(t, time.UTC, got.Location(), c.in)
	}
	for _, bad := range []string{"", "yesterday", "01/10/2026", "2026-13-01"} {
		_, err := parseTime(bad, false)
		assert.Error(t, err, bad)
	}
}

func TestPeriod(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

	from, to, err := period("", "", now, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, now, to)
	assert.Equal(t, now.Add(-24*time.Hour), from)

	from, to, err = period("2026-10-01", "", now, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), from)
	assert.Equal(t, now, to)

	// a single day
	from, to, err = period("2026-10-01", "2026-10-01", now, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour-time.Millisecond, to.Sub(from))

	_, _, err = period("2026-10-02", "2026-10-01", now, 24*time.Hour)
	assert.Error(t, err)
}

func TestLimit(t *testing.T) {
	assert.Equal(t, 100, limit(0, 100, 1000))
	assert.Equal(t, 100, limit(-5, 100, 1000))
	assert.Equal(t, 7, limit(7, 100, 1000))
	assert.Equal(t, 1000, limit(5000, 100, 1000))
}

func TestSample(t *testing.T) {
	values := make([]entity.TransactionMeter, 10)
	for i := range values {
		values[i].Value = i
	}
	picked := func(n int) []int {
		var out []int
		for _, v := range sample(values, n) {
			out = append(out, v.Value)
		}
		return out
	}
	assert.Equal(t, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, picked(0), "zero keeps everything")
	assert.Equal(t, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, picked(20))
	assert.Equal(t, []int{0, 9}, picked(2))
	assert.Equal(t, []int{9}, picked(1), "a single point is the latest")
	assert.Equal(t, []int{0, 2, 5, 7, 9}, picked(5))
	assert.Empty(t, sample(nil, 5))
}
