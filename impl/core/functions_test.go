package core

import (
	"evsys-back/entity"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeMeterValues(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		result := NormalizeMeterValues(nil, 60)
		assert.Nil(t, result)
	})

	t.Run("zero target length returns original", func(t *testing.T) {
		values := []*entity.TransactionMeter{
			{ConsumedEnergy: 100, PowerRate: 1000},
			{ConsumedEnergy: 200, PowerRate: 1500},
		}
		result := NormalizeMeterValues(values, 0)
		assert.Equal(t, values, result)
	})

	t.Run("length less than or equal to target returns original", func(t *testing.T) {
		values := []*entity.TransactionMeter{
			{ConsumedEnergy: 100, PowerRate: 1000},
			{ConsumedEnergy: 200, PowerRate: 1500},
		}
		result := NormalizeMeterValues(values, 60)
		assert.Equal(t, values, result)
	})

	t.Run("downsamples to target length", func(t *testing.T) {
		// Create 120 meter values
		values := make([]*entity.TransactionMeter, 120)
		for i := 0; i < 120; i++ {
			values[i] = &entity.TransactionMeter{
				ConsumedEnergy: i * 100,
				PowerRate:      7000,
			}
		}

		result := NormalizeMeterValues(values, 60)

		assert.Len(t, result, 60)
	})

	t.Run("interpolation for non-integer indices", func(t *testing.T) {
		values := []*entity.TransactionMeter{
			{ConsumedEnergy: 0, PowerRate: 0},
			{ConsumedEnergy: 100, PowerRate: 1000},
			{ConsumedEnergy: 200, PowerRate: 2000},
			{ConsumedEnergy: 300, PowerRate: 3000},
		}

		result := NormalizeMeterValues(values, 2)

		assert.Len(t, result, 2)
		// First value should be from index 0
		assert.Equal(t, 0, result[0].ConsumedEnergy)
		// Second value should be interpolated between indices 1 and 2
		// Index = 1 * (4/2) = 2, so it takes from index 2
	})

	t.Run("handles large downsample", func(t *testing.T) {
		// Create 600 meter values
		values := make([]*entity.TransactionMeter, 600)
		for i := 0; i < 600; i++ {
			values[i] = &entity.TransactionMeter{
				ConsumedEnergy: i * 10,
				PowerRate:      7200,
			}
		}

		result := NormalizeMeterValues(values, 60)

		assert.Len(t, result, 60)
		// First value should be from beginning
		assert.Equal(t, 0, result[0].ConsumedEnergy)
		// Last value should be from near end
		assert.True(t, result[59].ConsumedEnergy > 0)
	})
}

func TestInterpolate(t *testing.T) {
	tests := []struct {
		name     string
		x        float64
		x0       float64
		x1       float64
		y0       float64
		y1       float64
		expected float64
	}{
		{
			name:     "midpoint interpolation",
			x:        1.5,
			x0:       1.0,
			x1:       2.0,
			y0:       100.0,
			y1:       200.0,
			expected: 150.0,
		},
		{
			name:     "start point",
			x:        1.0,
			x0:       1.0,
			x1:       2.0,
			y0:       100.0,
			y1:       200.0,
			expected: 100.0,
		},
		{
			name:     "end point",
			x:        2.0,
			x0:       1.0,
			x1:       2.0,
			y0:       100.0,
			y1:       200.0,
			expected: 200.0,
		},
		{
			name:     "quarter point",
			x:        1.25,
			x0:       1.0,
			x1:       2.0,
			y0:       0.0,
			y1:       100.0,
			expected: 25.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interpolate(tt.x, tt.x0, tt.x1, tt.y0, tt.y1)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}
