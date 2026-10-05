package entity

import "time"

// LogFilter narrows a log query by time range, charge point and result size.
type LogFilter struct {
	From          *time.Time // Start of the period (inclusive)
	To            *time.Time // End of the period (inclusive)
	ChargePointId string     // Filter by charge point identifier, system and error logs only
	Limit         int64      // Maximum number of records to return
	// Search matches a case-insensitive substring of the record text: text on
	// log messages, info, error code and vendor error code on error records.
	Search string
	// Category matches exactly: feature on the system log (e.g.
	// StatusNotification), category on the backend and payment logs, error
	// code on the error log.
	Category string
	// Level matches exactly: importance on the system log, level on the
	// backend and payment logs. Not applicable to the error log.
	Level string
}

// HasFilters reports whether any narrowing criteria are set.
func (f *LogFilter) HasFilters() bool {
	return f != nil && (f.From != nil || f.To != nil || f.ChargePointId != "" || f.Limit > 0 ||
		f.Search != "" || f.Category != "" || f.Level != "")
}
