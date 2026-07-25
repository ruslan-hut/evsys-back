package entity

import "time"

// LogFilter narrows a log query by time range, charge point and result size.
type LogFilter struct {
	From          *time.Time // Start of the period (inclusive)
	To            *time.Time // End of the period (inclusive)
	ChargePointId string     // Filter by charge point identifier, system log only
	Limit         int64      // Maximum number of records to return
}

// HasFilters reports whether any narrowing criteria are set.
func (f *LogFilter) HasFilters() bool {
	return f != nil && (f.From != nil || f.To != nil || f.ChargePointId != "" || f.Limit > 0)
}
