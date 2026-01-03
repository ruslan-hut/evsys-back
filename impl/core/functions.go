package core

import (
	"evsys-back/entity"
	"fmt"
	"math"
)

func (c *Core) checkSubsystemAccess(user *entity.User, subsystem string) error {
	if c.auth == nil {
		return fmt.Errorf("authenticator not set")
	}
	err := c.auth.HasAccess(user, subsystem)
	if err != nil {
		return err
	}
	if subsystem == subSystemReports && c.reports == nil {
		return fmt.Errorf("report is not available")
	}
	return nil
}

func NormalizeMeterValues(meterValues []entity.TransactionMeter, newLength int) []entity.TransactionMeter {
	if newLength == 0 || meterValues == nil || len(meterValues) <= newLength {
		return meterValues
	}
	originalLength := len(meterValues)
	scaleFactor := float64(originalLength) / float64(newLength)
	normalized := make([]entity.TransactionMeter, newLength)
	for i := 0; i < newLength; i++ {
		originalIndex := float64(i) * scaleFactor
		lowerIndex := int(math.Floor(originalIndex))
		upperIndex := int(math.Ceil(originalIndex))
		if upperIndex >= originalLength {
			upperIndex = originalLength - 1
		}
		normalized[i] = meterValues[lowerIndex]
		if lowerIndex != upperIndex {
			v1 := float64(meterValues[lowerIndex].ConsumedEnergy)
			v2 := float64(meterValues[upperIndex].ConsumedEnergy)
			normalized[i].ConsumedEnergy = int(interpolate(originalIndex, float64(lowerIndex), float64(upperIndex), v1, v2))
			p1 := float64(meterValues[lowerIndex].PowerRate)
			p2 := float64(meterValues[upperIndex].PowerRate)
			normalized[i].PowerRate = int(interpolate(originalIndex, float64(lowerIndex), float64(upperIndex), p1, p2))
		}
	}
	return normalized
}

// Function to perform linear interpolation
func interpolate(x, x0, x1, y0, y1 float64) float64 {
	return y0 + (y1-y0)*(x-x0)/(x1-x0)
}
