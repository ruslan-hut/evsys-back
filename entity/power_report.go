package entity

// PowerGrouping selects how PowerStats rows are aggregated
type PowerGrouping string

const (
	PowerByCharger PowerGrouping = "charger"
	PowerBySession PowerGrouping = "session"
	PowerByHour    PowerGrouping = "hour"
	PowerByDay     PowerGrouping = "day"
)

// PowerGroupingFromString validates a group_by query parameter.
// An empty value defaults to grouping by charger.
func PowerGroupingFromString(s string) (PowerGrouping, bool) {
	switch PowerGrouping(s) {
	case "":
		return PowerByCharger, true
	case PowerByCharger, PowerBySession, PowerByHour, PowerByDay:
		return PowerGrouping(s), true
	default:
		return "", false
	}
}

// IsTimeline reports whether the grouping buckets samples over time rather
// than over sessions. Timeline rows describe fleet output; session and charger
// rows describe individual charging sessions.
func (g PowerGrouping) IsTimeline() bool {
	return g == PowerByHour || g == PowerByDay
}

// PowerStats is one row of the power report. Which identifying fields are set
// depends on the grouping: charger sets ChargePointId, session also sets
// TransactionId, and hour/day set Date (and Hour, for hourly buckets).
//
// Power is in watts and energy in watt-hours, matching the rest of the schema.
type PowerStats struct {
	ChargePointId string `json:"charge_point_id,omitempty" bson:"charge_point_id,omitempty"`
	TransactionId int    `json:"transaction_id,omitempty" bson:"transaction_id,omitempty"`
	Date          string `json:"date,omitempty" bson:"date,omitempty"`
	Hour          *int   `json:"hour,omitempty" bson:"hour,omitempty"`

	// Sessions is the number of charging sessions in the group. For timeline
	// groupings it counts the distinct sessions that delivered power in the bucket.
	Sessions int64 `json:"sessions" bson:"sessions"`

	// TotalConsumed is energy in watt-hours. For session and charger groupings
	// it is meter_stop - meter_start summed over sessions. For timeline
	// groupings it is integrated from the per-minute fleet power, so it
	// reflects energy actually delivered inside the bucket.
	TotalConsumed float64 `json:"total_consumed" bson:"total_consumed"`

	// DurationSeconds is the wall-clock length of the sessions in the group.
	// It is zero for timeline groupings, where the bucket defines the duration.
	DurationSeconds int64 `json:"duration_seconds" bson:"duration_seconds"`

	// AvgChargingPower is the sample-weighted mean of power_rate over samples
	// that were actually drawing power. For timeline groupings it is the mean
	// concurrent fleet power over the minutes in which the fleet was charging.
	AvgChargingPower float64 `json:"avg_charging_power" bson:"avg_charging_power"`

	// AvgSessionPower is energy divided by elapsed session time, so unlike
	// AvgChargingPower it is dragged down by idle time within a session. It is
	// zero for timeline groupings, where it has no meaning.
	AvgSessionPower float64 `json:"avg_session_power" bson:"avg_session_power"`

	// MaxPower is the peak of power_rate. For session and charger groupings it
	// is the highest single sample. For timeline groupings it is the highest
	// concurrent fleet draw, i.e. the peak load the site had to supply.
	MaxPower float64 `json:"max_power" bson:"max_power"`

	// Samples counts the meter values behind the power figures: charging
	// samples for session and charger rows, minute buckets for timeline rows.
	// Power figures are meaningless where this is zero.
	Samples int64 `json:"samples" bson:"samples"`
}
