package database

import (
	"context"
	"evsys-back/entity"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// transactionBasePipeline builds the shared aggregation prefix for transaction
// reports: filter by time range and positive consumption, resolve the user via
// id_tag -> user_tag -> user, filter by user group, and compute consumed_watts.
func transactionBasePipeline(from, to time.Time, userGroup string) mongo.Pipeline {
	return mongo.Pipeline{
		// Filter transactions by time range and positive consumption
		{{Key: "$match", Value: bson.D{
			{Key: "time_stop", Value: bson.D{
				{Key: "$gte", Value: from},
				{Key: "$lte", Value: to},
			}},
			{Key: "$expr", Value: bson.D{
				{Key: "$gt", Value: bson.A{"$meter_stop", "$meter_start"}},
			}},
		}}},
		// Lookup user tags by `id_tag`
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: collectionUserTags},
			{Key: "localField", Value: "id_tag"},
			{Key: "foreignField", Value: "id_tag"},
			{Key: "as", Value: "user_tag_info"},
		}}},
		// Add user id to the document
		{{Key: "$addFields", Value: bson.D{
			{Key: "user_id", Value: bson.D{
				{Key: "$cond", Value: bson.D{
					{Key: "if", Value: bson.D{{Key: "$gt", Value: bson.A{bson.D{{Key: "$size", Value: "$user_tag_info"}}, 0}}}},
					{Key: "then", Value: bson.D{{Key: "$arrayElemAt", Value: bson.A{"$user_tag_info.user_id", 0}}}},
					{Key: "else", Value: ""},
				}},
			}},
		}}},
		// Remove user_tag_info from the document
		{{Key: "$unset", Value: "user_tag_info"}},
		// Lookup users by `user_id` obtained from user_tag_info
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: collectionUsers},
			{Key: "localField", Value: "user_id"},
			{Key: "foreignField", Value: "user_id"},
			{Key: "as", Value: "user_info"},
		}}},
		// Unwind to de-nest user_info array
		{{Key: "$unwind", Value: "$user_info"}},
		// Filter transactions by a specific user group
		{{Key: "$match", Value: bson.D{
			{Key: "user_info.group", Value: userGroup},
		}}},
		// Calculate consumed watts
		{{Key: "$addFields", Value: bson.D{
			{Key: "consumed_watts", Value: bson.D{
				{Key: "$subtract", Value: bson.A{"$meter_stop", "$meter_start"}},
			}},
		}}},
	}
}

// aggregateReportLines runs a transactions aggregation pipeline and returns the
// decoded ReportLine documents as a generic slice.
func (m *MongoDB) aggregateReportLines(ctx context.Context, pipeline mongo.Pipeline) ([]any, error) {
	cursor, err := m.col(collectionTransactions).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	var lines []*ReportLine
	if err = cursor.All(ctx, &lines); err != nil {
		return nil, err
	}
	result := make([]any, len(lines))
	for i, v := range lines {
		result[i] = v
	}
	return result, nil
}

// TotalsByMonth returns the total consumed watts, average watts, and count of transactions by month
func (m *MongoDB) TotalsByMonth(ctx context.Context, from, to time.Time, userGroup string) ([]any, error) {
	pipeline := append(transactionBasePipeline(from, to, userGroup),
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "year", Value: bson.D{{Key: "$year", Value: "$time_stop"}}},
				{Key: "month", Value: bson.D{{Key: "$month", Value: "$time_stop"}}},
			}},
			{Key: "totalConsumed", Value: bson.D{{Key: "$sum", Value: "$consumed_watts"}}},
			{Key: "avgWatts", Value: bson.D{{Key: "$avg", Value: "$consumed_watts"}}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{
			{Key: "_id.year", Value: 1},
			{Key: "_id.month", Value: 1},
		}}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "year", Value: "$_id.year"},
			{Key: "month", Value: "$_id.month"},
			{Key: "totalConsumed", Value: 1},
			{Key: "avgWatts", Value: 1},
			{Key: "count", Value: 1},
		}}},
	)
	return m.aggregateReportLines(ctx, pipeline)
}

// TotalsByUsers returns the total consumed watts, average watts, and count of transactions by user
func (m *MongoDB) TotalsByUsers(ctx context.Context, from, to time.Time, userGroup string) ([]any, error) {
	pipeline := append(transactionBasePipeline(from, to, userGroup),
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$user_info.name"},
			{Key: "user", Value: bson.D{{Key: "$first", Value: "$user_info.name"}}},
			{Key: "totalConsumed", Value: bson.D{{Key: "$sum", Value: "$consumed_watts"}}},
			{Key: "avgWatts", Value: bson.D{{Key: "$avg", Value: "$consumed_watts"}}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{
			{Key: "user", Value: 1},
		}}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "user", Value: 1},
			{Key: "totalConsumed", Value: 1},
			{Key: "avgWatts", Value: 1},
			{Key: "count", Value: 1},
		}}},
	)
	return m.aggregateReportLines(ctx, pipeline)
}

func (m *MongoDB) TotalsByCharger(ctx context.Context, from, to time.Time, userGroup string) ([]any, error) {
	pipeline := append(transactionBasePipeline(from, to, userGroup),
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "charge_point", Value: "$charge_point_id"},
			}},
			{Key: "totalConsumed", Value: bson.D{{Key: "$sum", Value: "$consumed_watts"}}},
			{Key: "avgWatts", Value: bson.D{{Key: "$avg", Value: "$consumed_watts"}}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{
			{Key: "_id.charge_point", Value: 1},
		}}},
		// Reshape the output (uses "user" field for compatibility with ReportLine struct)
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "user", Value: "$_id.charge_point"},
			{Key: "totalConsumed", Value: 1},
			{Key: "avgWatts", Value: 1},
			{Key: "count", Value: 1},
		}}},
	)
	return m.aggregateReportLines(ctx, pipeline)
}

// TotalsByHour returns consumed energy grouped by date and hour (based on time_stop)
func (m *MongoDB) TotalsByHour(ctx context.Context, from, to time.Time, userGroup string) ([]any, error) {
	collection := m.col(collectionTransactions)

	pipeline := append(transactionBasePipeline(from, to, userGroup),
		// Group by date and hour of time_stop
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "date", Value: bson.D{{Key: "$dateToString", Value: bson.D{
					{Key: "format", Value: "%Y-%m-%d"},
					{Key: "date", Value: "$time_stop"},
				}}}},
				{Key: "hour", Value: bson.D{{Key: "$hour", Value: "$time_stop"}}},
			}},
			{Key: "consumed", Value: bson.D{{Key: "$sum", Value: "$consumed_watts"}}},
		}}},
		// Sort by date and hour
		bson.D{{Key: "$sort", Value: bson.D{
			{Key: "_id.date", Value: 1},
			{Key: "_id.hour", Value: 1},
		}}},
		// Reshape output
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "date", Value: "$_id.date"},
			{Key: "hour", Value: "$_id.hour"},
			{Key: "consumed", Value: 1},
		}}},
	)

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	var lines []bson.M
	if err = cursor.All(ctx, &lines); err != nil {
		return nil, err
	}

	// Build a complete set of 24 hours per date, filling missing hours with 0
	// Convert Wh to kWh by dividing by 1000
	hourlyData := make(map[string]map[int]float64)
	for _, line := range lines {
		date := line["date"].(string)
		hour := int(line["hour"].(int32))
		consumed := float64(0)
		switch v := line["consumed"].(type) {
		case int32:
			consumed = float64(v) / 1000
		case int64:
			consumed = float64(v) / 1000
		}
		if hourlyData[date] == nil {
			hourlyData[date] = make(map[int]float64)
		}
		hourlyData[date][hour] = consumed
	}

	// Generate all dates in range and fill 24 hours per date
	var dates []string
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}

	// Also include dates from query results that might not be in the generated range
	for date := range hourlyData {
		found := false
		for _, d := range dates {
			if d == date {
				found = true
				break
			}
		}
		if !found {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)

	var result []any
	for _, date := range dates {
		hours := hourlyData[date]
		for h := 0; h < 24; h++ {
			consumed := float64(0)
			if hours != nil {
				consumed = hours[h]
			}
			result = append(result, bson.M{
				"date":     date,
				"hour":     h,
				"consumed": consumed,
			})
		}
	}

	return result, nil
}

// reportTimezone is the zone the power timeline buckets hours and days in.
// TotalsByHour buckets in UTC implicitly; this keeps that behaviour but names
// it, so switching the fleet to local time is a one-line, deliberate change.
// It must stay an IANA name -- a fixed offset would break across DST.
const reportTimezone = "UTC"

// toDouble and toLong pin the numeric type of an aggregation result. The meter
// fields are Go ints and so land in BSON as int32; a $sum of them widens to
// int64 only once it overflows, which would otherwise make the decoded type
// depend on the size of the data.
func toDouble(expr any) bson.D {
	return bson.D{{Key: "$toDouble", Value: expr}}
}

func toLong(expr any) bson.D {
	return bson.D{{Key: "$toLong", Value: expr}}
}

// powerBasePipeline builds the shared prefix for the power report: finished
// sessions that consumed energy in the range, optionally narrowed to a single
// charge point and a single user group.
//
// Unlike transactionBasePipeline it joins users only when a group filter is
// actually requested. That join $unwinds user_info and so drops any session
// whose id_tag has no matching user, which would silently understate fleet
// production.
func powerBasePipeline(from, to time.Time, chargePointId, userGroup string) mongo.Pipeline {
	match := bson.D{
		{Key: "is_finished", Value: true},
		{Key: "time_stop", Value: bson.D{
			{Key: "$gte", Value: from},
			{Key: "$lte", Value: to},
		}},
		{Key: "$expr", Value: bson.D{
			{Key: "$gt", Value: bson.A{"$meter_stop", "$meter_start"}},
		}},
	}
	if chargePointId != "" {
		match = append(match, bson.E{Key: "charge_point_id", Value: chargePointId})
	}

	pipeline := mongo.Pipeline{{{Key: "$match", Value: match}}}
	if userGroup == "" {
		return pipeline
	}

	return append(pipeline,
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: collectionUserTags},
			{Key: "localField", Value: "id_tag"},
			{Key: "foreignField", Value: "id_tag"},
			{Key: "as", Value: "user_tag_info"},
		}}},
		bson.D{{Key: "$addFields", Value: bson.D{
			{Key: "user_id", Value: bson.D{
				{Key: "$arrayElemAt", Value: bson.A{"$user_tag_info.user_id", 0}},
			}},
		}}},
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: collectionUsers},
			{Key: "localField", Value: "user_id"},
			{Key: "foreignField", Value: "user_id"},
			{Key: "as", Value: "user_info"},
		}}},
		bson.D{{Key: "$unwind", Value: "$user_info"}},
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "user_info.group", Value: userGroup},
		}}},
		bson.D{{Key: "$unset", Value: bson.A{"user_tag_info", "user_info"}}},
	)
}

// powerSessionStages derives per-session power figures from the embedded
// meter_values array. Samples drawing no power are dropped up front so that
// idle time inside a session does not deflate the charging average; it is
// still reflected in avg_session_power, which divides by elapsed time.
//
// Two stages are needed because charging_samples cannot be referenced from the
// same $addFields that creates it.
func powerSessionStages() []bson.D {
	return []bson.D{
		{{Key: "$addFields", Value: bson.D{
			{Key: "consumed_watts", Value: bson.D{
				{Key: "$subtract", Value: bson.A{"$meter_stop", "$meter_start"}},
			}},
			{Key: "duration_seconds", Value: bson.D{
				{Key: "$dateDiff", Value: bson.D{
					{Key: "startDate", Value: "$time_start"},
					{Key: "endDate", Value: "$time_stop"},
					{Key: "unit", Value: "second"},
				}},
			}},
			{Key: "charging_samples", Value: bson.D{
				{Key: "$filter", Value: bson.D{
					{Key: "input", Value: bson.D{
						{Key: "$ifNull", Value: bson.A{"$meter_values", bson.A{}}},
					}},
					{Key: "as", Value: "mv"},
					{Key: "cond", Value: bson.D{
						{Key: "$gt", Value: bson.A{"$$mv.power_rate", 0}},
					}},
				}},
			}},
		}}},
		// $max over an empty array yields null, which would poison the $max
		// accumulator downstream, so coalesce it to 0 here.
		{{Key: "$addFields", Value: bson.D{
			{Key: "session_max_power", Value: bson.D{
				{Key: "$ifNull", Value: bson.A{
					bson.D{{Key: "$max", Value: "$charging_samples.power_rate"}},
					0,
				}},
			}},
			{Key: "power_sum", Value: bson.D{
				{Key: "$sum", Value: "$charging_samples.power_rate"},
			}},
			{Key: "power_count", Value: bson.D{
				{Key: "$size", Value: "$charging_samples"},
			}},
		}}},
	}
}

// avgSessionPowerExpr converts summed energy and elapsed time into average
// watts: Wh * 3600 / seconds.
//
// The guard is load-bearing, not defensive: $divide by zero aborts the whole
// aggregation rather than yielding null, and sessions that start and stop
// within the same second do occur. Dividing before multiplying keeps the
// arithmetic in double and out of int64 overflow range.
func avgSessionPowerExpr(consumed, seconds string) bson.D {
	return bson.D{{Key: "$cond", Value: bson.D{
		{Key: "if", Value: bson.D{{Key: "$gt", Value: bson.A{seconds, 0}}}},
		{Key: "then", Value: bson.D{{Key: "$multiply", Value: bson.A{
			bson.D{{Key: "$divide", Value: bson.A{toDouble(consumed), seconds}}},
			3600,
		}}}},
		{Key: "else", Value: float64(0)},
	}}}
}

// safeDivideExpr guards against dividing by a zero count, which happens for
// sessions that recorded no charging samples at all. The else branch is
// explicitly a float so the field decodes into a float64 either way.
func safeDivideExpr(sum, count string) bson.D {
	return bson.D{{Key: "$cond", Value: bson.D{
		{Key: "if", Value: bson.D{{Key: "$gt", Value: bson.A{count, 0}}}},
		{Key: "then", Value: bson.D{{Key: "$divide", Value: bson.A{toDouble(sum), count}}}},
		{Key: "else", Value: float64(0)},
	}}}
}

// powerByChargerPipeline aggregates sessions into one row per charge point.
// The charging average is sample-weighted: per-session sums and counts are
// added up and divided once at the end, so a long session counts for more than
// a short one instead of every session's mean carrying equal weight.
func powerByChargerPipeline(from, to time.Time, chargePointId, userGroup string) mongo.Pipeline {
	pipeline := append(powerBasePipeline(from, to, chargePointId, userGroup), powerSessionStages()...)
	return append(pipeline,
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$charge_point_id"},
			{Key: "sessions", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "total_consumed", Value: bson.D{{Key: "$sum", Value: "$consumed_watts"}}},
			{Key: "duration_seconds", Value: bson.D{{Key: "$sum", Value: "$duration_seconds"}}},
			{Key: "max_power", Value: bson.D{{Key: "$max", Value: "$session_max_power"}}},
			{Key: "power_sum", Value: bson.D{{Key: "$sum", Value: "$power_sum"}}},
			{Key: "samples", Value: bson.D{{Key: "$sum", Value: "$power_count"}}},
		}}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "charge_point_id", Value: "$_id"},
			{Key: "sessions", Value: toLong("$sessions")},
			{Key: "total_consumed", Value: toDouble("$total_consumed")},
			{Key: "duration_seconds", Value: toLong("$duration_seconds")},
			{Key: "max_power", Value: toDouble("$max_power")},
			{Key: "samples", Value: toLong("$samples")},
			{Key: "avg_charging_power", Value: safeDivideExpr("$power_sum", "$samples")},
			{Key: "avg_session_power", Value: avgSessionPowerExpr("$total_consumed", "$duration_seconds")},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "charge_point_id", Value: 1}}}},
	)
}

// powerBySessionPipeline emits one row per charging session. No $group is
// needed: each transaction document is already the unit of aggregation.
func powerBySessionPipeline(from, to time.Time, chargePointId, userGroup string) mongo.Pipeline {
	pipeline := append(powerBasePipeline(from, to, chargePointId, userGroup), powerSessionStages()...)
	return append(pipeline,
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "transaction_id", Value: 1},
			{Key: "charge_point_id", Value: 1},
			{Key: "sessions", Value: bson.D{{Key: "$literal", Value: int64(1)}}},
			{Key: "total_consumed", Value: toDouble("$consumed_watts")},
			{Key: "duration_seconds", Value: toLong("$duration_seconds")},
			{Key: "max_power", Value: toDouble("$session_max_power")},
			{Key: "samples", Value: toLong("$power_count")},
			{Key: "avg_charging_power", Value: safeDivideExpr("$power_sum", "$power_count")},
			{Key: "avg_session_power", Value: avgSessionPowerExpr("$consumed_watts", "$duration_seconds")},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "max_power", Value: -1}}}},
	)
}

// powerTimelinePipeline measures fleet output over time.
//
// Power at an instant is the *sum* of every session drawing at that instant, so
// samples are collapsed per minute and then summed across sessions to
// reconstruct the concurrent fleet draw. Only then is that per-minute series
// bucketed, which is what makes max_power a true peak load rather than the best
// any single charger managed. Energy is integrated from the same series and so
// is credited to the bucket it was delivered in, rather than to the hour the
// session happened to stop the way TotalsByHour does it.
func powerTimelinePipeline(from, to time.Time, chargePointId, userGroup string, unit string) mongo.Pipeline {
	pipeline := append(powerBasePipeline(from, to, chargePointId, userGroup),
		// Drop the payment orders, tariffs and unused meter fields before the
		// array explodes; this is the cheapest win in the whole pipeline.
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "transaction_id", Value: 1},
			{Key: "meter_values.power_rate", Value: 1},
			{Key: "meter_values.time", Value: 1},
		}}},
		bson.D{{Key: "$unwind", Value: "$meter_values"}},
		// Samples can predate `from` when a session started before the window,
		// so bucket on sample time to keep the series inside the range. The
		// $type guard matters: $dateTrunc on a non-date aborts the aggregation.
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "meter_values.power_rate", Value: bson.D{{Key: "$gt", Value: 0}}},
			{Key: "meter_values.time", Value: bson.D{
				{Key: "$type", Value: "date"},
				{Key: "$gte", Value: from},
				{Key: "$lte", Value: to},
			}},
		}}},
		// One row per session-minute. Without this, a session that reports more
		// than one sample in a minute (a second measurand, or a sub-minute
		// sample interval) would be counted repeatedly and inflate fleet power.
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "txn", Value: "$transaction_id"},
				{Key: "minute", Value: bson.D{{Key: "$dateTrunc", Value: bson.D{
					{Key: "date", Value: "$meter_values.time"},
					{Key: "unit", Value: "minute"},
				}}}},
			}},
			{Key: "power", Value: bson.D{{Key: "$max", Value: "$meter_values.power_rate"}}},
		}}},
		// Concurrent fleet draw per minute.
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$_id.minute"},
			{Key: "fleet_power", Value: bson.D{{Key: "$sum", Value: "$power"}}},
			{Key: "transactions", Value: bson.D{{Key: "$addToSet", Value: "$_id.txn"}}},
		}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "$dateTrunc", Value: bson.D{
				{Key: "date", Value: "$_id"},
				{Key: "unit", Value: unit},
				{Key: "timezone", Value: reportTimezone},
			}}}},
			{Key: "max_power", Value: bson.D{{Key: "$max", Value: "$fleet_power"}}},
			{Key: "power_sum", Value: bson.D{{Key: "$sum", Value: "$fleet_power"}}},
			{Key: "samples", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "transaction_sets", Value: bson.D{{Key: "$push", Value: "$transactions"}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	)

	project := bson.D{
		{Key: "_id", Value: 0},
		{Key: "date", Value: bson.D{{Key: "$dateToString", Value: bson.D{
			{Key: "format", Value: "%Y-%m-%d"},
			{Key: "date", Value: "$_id"},
			{Key: "timezone", Value: reportTimezone},
		}}}},
		{Key: "max_power", Value: toDouble("$max_power")},
		{Key: "samples", Value: toLong("$samples")},
		// Seconds the fleet spent charging in this bucket, i.e. charging
		// minutes scaled up -- not the bucket's own length.
		{Key: "duration_seconds", Value: toLong(bson.D{
			{Key: "$multiply", Value: bson.A{"$samples", 60}},
		})},
		// Each charging minute contributes power/60 Wh. This assumes samples
		// land about once a minute, which is what the chargers are configured
		// for; a coarser MeterValueSampleInterval would leave un-sampled
		// minutes out of the sum and understate energy. The reconciliation
		// check in the plan (timeline total vs charger total) is what catches
		// that, since nothing here can observe the charger's setting.
		{Key: "total_consumed", Value: bson.D{{Key: "$divide", Value: bson.A{"$power_sum", 60}}}},
		// Mean fleet draw across the minutes the fleet was actually charging.
		// Idle minutes produce no bucket at all, so they are absent from the
		// denominator by construction: this is not mean load over the bucket,
		// which is simply total_consumed / bucket hours.
		{Key: "avg_charging_power", Value: safeDivideExpr("$power_sum", "$samples")},
		// $literal, not a bare 0: a plain number in a $project is an
		// include/exclude flag, and mixing an exclusion into an inclusion
		// projection is rejected outright.
		{Key: "avg_session_power", Value: bson.D{{Key: "$literal", Value: float64(0)}}},
		// A session spanning several buckets is counted in each one, so this
		// column does not sum to a period total.
		{Key: "sessions", Value: toLong(bson.D{{Key: "$size", Value: bson.D{
			{Key: "$reduce", Value: bson.D{
				{Key: "input", Value: "$transaction_sets"},
				{Key: "initialValue", Value: bson.A{}},
				{Key: "in", Value: bson.D{
					{Key: "$setUnion", Value: bson.A{"$$value", "$$this"}},
				}},
			}},
		}}})},
	}
	if unit == "hour" {
		project = append(project, bson.E{Key: "hour", Value: bson.D{{Key: "$hour", Value: bson.D{
			{Key: "date", Value: "$_id"},
			{Key: "timezone", Value: reportTimezone},
		}}}})
	}

	return append(pipeline, bson.D{{Key: "$project", Value: project}})
}

// PowerStats aggregates instantaneous power (power_rate, watts) from the
// meter_values embedded in finished transactions.
//
// It reads the raw stored array rather than the API's meter values, which are
// downsampled to a fixed number of points and interpolated, so the peaks here
// are exact.
func (m *MongoDB) PowerStats(ctx context.Context, from, to time.Time, chargePointId, userGroup, groupBy string) ([]*entity.PowerStats, error) {
	grouping, ok := entity.PowerGroupingFromString(groupBy)
	if !ok {
		return nil, fmt.Errorf("unknown grouping %q", groupBy)
	}

	var pipeline mongo.Pipeline
	switch grouping {
	case entity.PowerBySession:
		pipeline = powerBySessionPipeline(from, to, chargePointId, userGroup)
	case entity.PowerByHour:
		pipeline = powerTimelinePipeline(from, to, chargePointId, userGroup, "hour")
	case entity.PowerByDay:
		pipeline = powerTimelinePipeline(from, to, chargePointId, userGroup, "day")
	default:
		pipeline = powerByChargerPipeline(from, to, chargePointId, userGroup)
	}

	// The timeline groupings unwind every sample in the range before collapsing
	// them, so let the server spill rather than fail on a wide date range.
	opts := options.Aggregate()
	if grouping.IsTimeline() {
		opts.SetAllowDiskUse(true)
	}

	return aggregateMany[*entity.PowerStats](m, ctx, collectionTransactions, pipeline, opts)
}

// StationUptime calculates uptime/downtime for stations over a period
// based on registered/unregistered events in sys_log
// Only includes charge points that exist in charge_points collection and are enabled
func (m *MongoDB) StationUptime(ctx context.Context, from, to time.Time, chargePointId string) ([]*entity.StationUptime, error) {
	// Get list of enabled charge point IDs
	enabledCPs, err := m.getEnabledChargePointIds(ctx, chargePointId)
	if err != nil {
		return nil, err
	}
	if len(enabledCPs) == 0 {
		return []*entity.StationUptime{}, nil
	}

	collection := m.col(collectionSysLog)

	// Build base filter for events containing "registered" and matching enabled charge points
	baseFilter := bson.D{
		{Key: "text", Value: bson.D{{Key: "$regex", Value: "registered"}}},
		{Key: "charge_point_id", Value: bson.D{{Key: "$in", Value: enabledCPs}}},
	}

	// Find earliest record timestamp and adjust 'from' if needed
	earliestOpts := options.FindOne().SetSort(bson.D{{Key: "timestamp", Value: 1}}).SetProjection(bson.D{{Key: "timestamp", Value: 1}})
	var earliest struct {
		Timestamp time.Time `bson:"timestamp"`
	}
	err = collection.FindOne(ctx, baseFilter, earliestOpts).Decode(&earliest)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return []*entity.StationUptime{}, nil
		}
		return nil, m.findError(err)
	}

	// Limit 'from' to earliest available data
	if from.Before(earliest.Timestamp) {
		from = earliest.Timestamp
	}

	// Get all relevant events sorted by charge_point_id and timestamp
	opts := options.Find().SetSort(bson.D{{Key: "charge_point_id", Value: 1}, {Key: "timestamp", Value: 1}})
	cursor, err := collection.Find(ctx, baseFilter, opts)
	if err != nil {
		return nil, m.findError(err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		_ = cursor.Close(ctx)
	}(cursor, ctx)

	// Parse events
	var events []struct {
		ChargePointId string    `bson:"charge_point_id"`
		Text          string    `bson:"text"`
		Timestamp     time.Time `bson:"timestamp"`
	}
	if err = cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	// Group events by charge_point_id
	eventsByStation := make(map[string][]struct {
		Text      string
		Timestamp time.Time
	})
	for _, e := range events {
		eventsByStation[e.ChargePointId] = append(eventsByStation[e.ChargePointId], struct {
			Text      string
			Timestamp time.Time
		}{e.Text, e.Timestamp})
	}

	// Calculate uptime for each station
	var results []*entity.StationUptime
	for cpId, stationEvents := range eventsByStation {
		uptime := &entity.StationUptime{
			ChargePointId: cpId,
			FinalState:    entity.StateUnknown,
		}

		// Find initial state (last event before 'from')
		currentState := entity.StateOffline // Default assumption
		lastTime := from

		for _, e := range stationEvents {
			// Events before 'from' establish initial state
			if e.Timestamp.Before(from) {
				currentState = entity.StateFromText(e.Text)
				continue
			}

			// Events after 'to' are ignored
			if e.Timestamp.After(to) {
				break
			}

			// Calculate duration in previous state
			duration := e.Timestamp.Sub(lastTime)
			if currentState == entity.StateOnline {
				uptime.OnlineDuration += duration
			} else {
				uptime.OfflineDuration += duration
			}

			// Transition to new state
			currentState = entity.StateFromText(e.Text)
			lastTime = e.Timestamp
		}

		// Add tail interval from last event to 'to'
		tailDuration := to.Sub(lastTime)
		if currentState == entity.StateOnline {
			uptime.OnlineDuration += tailDuration
		} else {
			uptime.OfflineDuration += tailDuration
		}

		uptime.FinalState = currentState

		// Calculate uptime percentage
		totalDuration := uptime.OnlineDuration + uptime.OfflineDuration
		if totalDuration > 0 {
			uptime.UptimePercent = float64(uptime.OnlineDuration) / float64(totalDuration) * 100
		}

		results = append(results, uptime)
	}

	// Sort results by charge_point_id
	sort.Slice(results, func(i, j int) bool {
		return results[i].ChargePointId < results[j].ChargePointId
	})

	return results, nil
}

// StationStatus returns the current connection state for stations
// based on the most recent registered/unregistered event
// Only includes charge points that exist in charge_points collection and are enabled
func (m *MongoDB) StationStatus(ctx context.Context, chargePointId string) ([]*entity.StationStatus, error) {
	// Get list of enabled charge point IDs
	enabledCPs, err := m.getEnabledChargePointIds(ctx, chargePointId)
	if err != nil {
		return nil, err
	}
	if len(enabledCPs) == 0 {
		return []*entity.StationStatus{}, nil
	}

	collection := m.col(collectionSysLog)

	// Build match stage with enabled charge points filter
	matchStage := bson.D{
		{Key: "$match", Value: bson.D{
			{Key: "text", Value: bson.D{{Key: "$regex", Value: "registered"}}},
			{Key: "charge_point_id", Value: bson.D{{Key: "$in", Value: enabledCPs}}},
		}},
	}

	pipeline := mongo.Pipeline{
		matchStage,
		// Sort by charge_point_id and timestamp descending
		{{Key: "$sort", Value: bson.D{
			{Key: "charge_point_id", Value: 1},
			{Key: "timestamp", Value: -1},
		}}},
		// Group by charge_point_id, take the first (most recent) event
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$charge_point_id"},
			{Key: "text", Value: bson.D{{Key: "$first", Value: "$text"}}},
			{Key: "timestamp", Value: bson.D{{Key: "$first", Value: "$timestamp"}}},
		}}},
		// Sort by charge_point_id
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		_ = cursor.Close(ctx)
	}(cursor, ctx)

	var docs []struct {
		ChargePointId string    `bson:"_id"`
		Text          string    `bson:"text"`
		Timestamp     time.Time `bson:"timestamp"`
	}
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	now := time.Now()
	results := make([]*entity.StationStatus, len(docs))
	for i, doc := range docs {
		results[i] = &entity.StationStatus{
			ChargePointId: doc.ChargePointId,
			State:         entity.StateFromText(doc.Text),
			Since:         doc.Timestamp,
			Duration:      now.Sub(doc.Timestamp),
			LastEventText: doc.Text,
		}
	}

	return results, nil
}

// getEnabledChargePointIds returns IDs of charge points that are enabled
// If chargePointId is specified, returns only that ID if it's enabled
func (m *MongoDB) getEnabledChargePointIds(ctx context.Context, chargePointId string) ([]string, error) {
	collection := m.col(collectionChargePoints)

	filter := bson.D{{Key: "is_enabled", Value: true}}
	if chargePointId != "" {
		filter = append(filter, bson.E{Key: "charge_point_id", Value: chargePointId})
	}

	// Only fetch the charge_point_id field
	opts := options.Find().SetProjection(bson.D{{Key: "charge_point_id", Value: 1}})
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, m.findError(err)
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		_ = cursor.Close(ctx)
	}(cursor, ctx)

	var docs []struct {
		ChargePointId string `bson:"charge_point_id"`
	}
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	ids := make([]string, len(docs))
	for i, doc := range docs {
		ids[i] = doc.ChargePointId
	}
	return ids, nil
}
