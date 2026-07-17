package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPowerGroupingFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected PowerGrouping
		valid    bool
	}{
		{
			name:     "empty defaults to charger",
			input:    "",
			expected: PowerByCharger,
			valid:    true,
		},
		{
			name:     "charger",
			input:    "charger",
			expected: PowerByCharger,
			valid:    true,
		},
		{
			name:     "session",
			input:    "session",
			expected: PowerBySession,
			valid:    true,
		},
		{
			name:     "hour",
			input:    "hour",
			expected: PowerByHour,
			valid:    true,
		},
		{
			name:     "day",
			input:    "day",
			expected: PowerByDay,
			valid:    true,
		},
		{
			name:  "unknown value is rejected",
			input: "week",
			valid: false,
		},
		{
			name:  "grouping is case sensitive",
			input: "Charger",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grouping, ok := PowerGroupingFromString(tt.input)
			assert.Equal(t, tt.valid, ok)
			if tt.valid {
				assert.Equal(t, tt.expected, grouping)
			}
		})
	}
}

func TestPowerGrouping_IsTimeline(t *testing.T) {
	// Timeline groupings report fleet output over time, so their max_power is a
	// concurrent total rather than a single session's peak.
	assert.True(t, PowerByHour.IsTimeline())
	assert.True(t, PowerByDay.IsTimeline())
	assert.False(t, PowerByCharger.IsTimeline())
	assert.False(t, PowerBySession.IsTimeline())
}
