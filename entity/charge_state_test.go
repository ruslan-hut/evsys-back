package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChargeState_CheckState(t *testing.T) {
	tests := []struct {
		name          string
		state         ChargeState
		expectedPower int
	}{
		{
			name: "IsCharging true and Status Charging - keeps PowerRate",
			state: ChargeState{
				IsCharging: true,
				Status:     "Charging",
				PowerRate:  7200,
			},
			expectedPower: 7200,
		},
		{
			name: "IsCharging false - clears PowerRate",
			state: ChargeState{
				IsCharging: false,
				Status:     "Charging",
				PowerRate:  7200,
			},
			expectedPower: 0,
		},
		{
			name: "IsCharging true but Status not Charging - clears PowerRate",
			state: ChargeState{
				IsCharging: true,
				Status:     "Finishing",
				PowerRate:  7200,
			},
			expectedPower: 0,
		},
		{
			name: "Both conditions false - clears PowerRate",
			state: ChargeState{
				IsCharging: false,
				Status:     "Available",
				PowerRate:  7200,
			},
			expectedPower: 0,
		},
		{
			name: "Zero PowerRate stays zero",
			state: ChargeState{
				IsCharging: false,
				Status:     "Available",
				PowerRate:  0,
			},
			expectedPower: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.state.CheckState()
			assert.Equal(t, tt.expectedPower, tt.state.PowerRate)
		})
	}
}
