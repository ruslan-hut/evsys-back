package database

import (
	"context"
	"evsys-back/entity"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sort"
	"time"
)

// TotalsByMonth returns the total consumed watts, average watts, and count of transactions by month
func (m *MongoDB) TotalsByMonth(ctx context.Context, from, to time.Time, userGroup string) ([]interface{}, error) {
	collection := m.client.Database(m.database).Collection(collectionTransactions)

	pipeline := mongo.Pipeline{
		// Filter transactions
		{{"$match", bson.D{
			{"time_stop", bson.D{
				{"$gte", from},
				{"$lte", to},
			}},
			{"$expr", bson.D{
				{"$gt", bson.A{"$meter_stop", "$meter_start"}},
			}},
		}}},
		// Lookup user tags by `id_tag`
		{{"$lookup", bson.D{
			{"from", collectionUserTags},
			{"localField", "id_tag"},
			{"foreignField", "id_tag"},
			{"as", "user_tag_info"},
		}}},
		// Add user id to the document
		{{"$addFields", bson.D{
			{"user_id", bson.D{
				{"$cond", bson.D{
					{"if", bson.D{{"$gt", bson.A{bson.D{{"$size", "$user_tag_info"}}, 0}}}},
					{"then", bson.D{{"$arrayElemAt", bson.A{"$user_tag_info.user_id", 0}}}},
					{"else", ""},
				}},
			}},
		}}},
		// Unwind to de-nest user_tag_info array
		//{{"$unwind", "$user_tag_info"}},
		// Remove user_tag_info from the document
		{{"$unset", "user_tag_info"}},
		// Lookup users by `user_id` obtained from `user_tag_info`
		{{"$lookup", bson.D{
			{"from", collectionUsers},
			{"localField", "user_id"},
			{"foreignField", "user_id"},
			{"as", "user_info"},
		}}},
		// Unwind to de-nest user_info array
		{{"$unwind", "$user_info"}},
		// Stage 5: Filter transactions by a specific user group
		{{"$match", bson.D{
			{"user_info.group", userGroup},
		}}},
		// Calculate consumed watts and group by year and month
		{{"$addFields", bson.D{
			{"consumed_watts", bson.D{
				{"$subtract", bson.A{"$meter_stop", "$meter_start"}},
			}},
		}}},
		{{"$group", bson.D{
			{"_id", bson.D{
				{"year", bson.D{{"$year", "$time_stop"}}},
				{"month", bson.D{{"$month", "$time_stop"}}},
			}},
			{"totalConsumed", bson.D{{"$sum", "$consumed_watts"}}},
			{"avgWatts", bson.D{{"$avg", "$consumed_watts"}}},
			{"count", bson.D{{"$sum", 1}}},
		}}},
		// Sort by year and month
		{{"$sort", bson.D{
			{"_id.year", 1},
			{"_id.month", 1},
		}}},
		// Reshape the output if needed
		{{"$project", bson.D{
			{"_id", 0},
			{"year", "$_id.year"},
			{"month", "$_id.month"},
			{"totalConsumed", 1},
			{"avgWatts", 1},
			{"count", 1},
		}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	var lines []*ReportLine
	if err = cursor.All(ctx, &lines); err != nil {
		return nil, err
	}
	result := make([]interface{}, len(lines))
	for i, v := range lines {
		result[i] = v
	}
	return result, err
}

// TotalsByUsers returns the total consumed watts, average watts, and count of transactions by user
func (m *MongoDB) TotalsByUsers(ctx context.Context, from, to time.Time, userGroup string) ([]interface{}, error) {
	collection := m.client.Database(m.database).Collection(collectionTransactions)

	pipeline := mongo.Pipeline{
		// Stage 0: Filter transactions
		{{"$match", bson.D{
			{"time_stop", bson.D{
				{"$gte", from},
				{"$lte", to},
			}},
			{"$expr", bson.D{
				{"$gt", bson.A{"$meter_stop", "$meter_start"}},
			}},
		}}},
		// Stage 1: Lookup user tags by `id_tag`
		{{"$lookup", bson.D{
			{"from", collectionUserTags},
			{"localField", "id_tag"},
			{"foreignField", "id_tag"},
			{"as", "user_tag_info"},
		}}},
		// Add user id to the document
		{{"$addFields", bson.D{
			{"user_id", bson.D{
				{"$cond", bson.D{
					{"if", bson.D{{"$gt", bson.A{bson.D{{"$size", "$user_tag_info"}}, 0}}}},
					{"then", bson.D{{"$arrayElemAt", bson.A{"$user_tag_info.user_id", 0}}}},
					{"else", ""},
				}},
			}},
		}}},
		// Stage 2: Unwind to de-nest user_tag_info array
		//{{"$unwind", "$user_tag_info"}},
		// Remove user_tag_info from the document
		{{"$unset", "user_tag_info"}},
		// Stage 3: Lookup users by `user_id` obtained from `user_tag_info`
		{{"$lookup", bson.D{
			{"from", collectionUsers},
			{"localField", "user_id"},
			{"foreignField", "user_id"},
			{"as", "user_info"},
		}}},
		// Stage 4: Unwind to de-nest user_info array
		{{"$unwind", "$user_info"}},
		// Stage 5: Filter transactions by a specific user group
		{{"$match", bson.D{
			{"user_info.group", userGroup},
		}}},
		// Stage 6: Calculate consumed watts and group by year and month
		{{"$addFields", bson.D{
			{"consumed_watts", bson.D{
				{"$subtract", bson.A{"$meter_stop", "$meter_start"}},
			}},
		}}},
		{{"$group", bson.D{
			{"_id", bson.D{
				//{"year", bson.D{{"$year", "$time_stop"}}},
				//{"month", bson.D{{"$month", "$time_stop"}}},
				{"user", "$user_info.name"},
			}},
			{"totalConsumed", bson.D{{"$sum", "$consumed_watts"}}},
			{"avgWatts", bson.D{{"$avg", "$consumed_watts"}}},
			{"count", bson.D{{"$sum", 1}}},
		}}},
		// Stage 7: Sort by year and month
		{{"$sort", bson.D{
			//{"_id.year", 1},
			//{"_id.month", 1},
			{"_id.user", 1},
		}}},
		// (Optional) Stage 8: Reshape the output if needed
		{{"$project", bson.D{
			{"_id", 0},
			//{"year", "$_id.year"},
			//{"month", "$_id.month"},
			{"user", "$_id.user"},
			{"totalConsumed", 1},
			{"avgWatts", 1},
			{"count", 1},
		}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	var lines []*ReportLine
	if err = cursor.All(ctx, &lines); err != nil {
		return nil, err
	}
	result := make([]interface{}, len(lines))
	for i, v := range lines {
		result[i] = v
	}
	return result, err
}

func (m *MongoDB) TotalsByCharger(ctx context.Context, from, to time.Time, userGroup string) ([]interface{}, error) {
	collection := m.client.Database(m.database).Collection(collectionTransactions)

	pipeline := mongo.Pipeline{
		// Stage 0: Filter transactions
		{{"$match", bson.D{
			{"time_stop", bson.D{
				{"$gte", from},
				{"$lte", to},
			}},
			{"$expr", bson.D{
				{"$gt", bson.A{"$meter_stop", "$meter_start"}},
			}},
		}}},
		// Stage 1: Lookup user tags by `id_tag`
		{{"$lookup", bson.D{
			{"from", collectionUserTags},
			{"localField", "id_tag"},
			{"foreignField", "id_tag"},
			{"as", "user_tag_info"},
		}}},
		// Add user id to the document
		{{"$addFields", bson.D{
			{"user_id", bson.D{
				{"$cond", bson.D{
					{"if", bson.D{{"$gt", bson.A{bson.D{{"$size", "$user_tag_info"}}, 0}}}},
					{"then", bson.D{{"$arrayElemAt", bson.A{"$user_tag_info.user_id", 0}}}},
					{"else", ""},
				}},
			}},
		}}},
		// Stage 2: Unwind to de-nest user_tag_info array
		//{{"$unwind", "$user_tag_info"}},
		// Remove user_tag_info from the document
		{{"$unset", "user_tag_info"}},
		// Stage 3: Lookup users by `user_id` obtained from `user_tag_info`
		{{"$lookup", bson.D{
			{"from", collectionUsers},
			{"localField", "user_id"},
			{"foreignField", "user_id"},
			{"as", "user_info"},
		}}},
		// Stage 4: Unwind to de-nest user_info array
		{{"$unwind", "$user_info"}},
		// Stage 5: Filter transactions by a specific user group
		{{"$match", bson.D{
			{"user_info.group", userGroup},
		}}},
		// Stage 6: Calculate consumed watts and group by year and month
		{{"$addFields", bson.D{
			{"consumed_watts", bson.D{
				{"$subtract", bson.A{"$meter_stop", "$meter_start"}},
			}},
		}}},
		{{"$group", bson.D{
			{"_id", bson.D{
				//{"year", bson.D{{"$year", "$time_stop"}}},
				//{"month", bson.D{{"$month", "$time_stop"}}},
				{"charge_point", "$charge_point_id"},
			}},
			{"totalConsumed", bson.D{{"$sum", "$consumed_watts"}}},
			{"avgWatts", bson.D{{"$avg", "$consumed_watts"}}},
			{"count", bson.D{{"$sum", 1}}},
		}}},
		// Stage 7: Sort by year and month
		{{"$sort", bson.D{
			//{"_id.year", 1},
			//{"_id.month", 1},
			{"_id.charge_point", 1},
		}}},
		// (Optional) Stage 8: Reshape the output if needed
		{{"$project", bson.D{
			{"_id", 0},
			//{"year", "$_id.year"},
			//{"month", "$_id.month"},
			{"user", "$_id.charge_point"},
			{"totalConsumed", 1},
			{"avgWatts", 1},
			{"count", 1},
		}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	var lines []*ReportLine
	if err = cursor.All(ctx, &lines); err != nil {
		return nil, err
	}
	result := make([]interface{}, len(lines))
	for i, v := range lines {
		result[i] = v
	}
	return result, err
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

	collection := m.client.Database(m.database).Collection(collectionSysLog)

	// Build base filter for events containing "registered" and matching enabled charge points
	baseFilter := bson.D{
		{"text", bson.D{{"$regex", "registered"}}},
		{"charge_point_id", bson.D{{"$in", enabledCPs}}},
	}

	// Find earliest record timestamp and adjust 'from' if needed
	earliestOpts := options.FindOne().SetSort(bson.D{{"timestamp", 1}}).SetProjection(bson.D{{"timestamp", 1}})
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
	opts := options.Find().SetSort(bson.D{{"charge_point_id", 1}, {"timestamp", 1}})
	cursor, err := collection.Find(ctx, baseFilter, opts)
	if err != nil {
		return nil, m.findError(err)
	}
	defer cursor.Close(ctx)

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

	collection := m.client.Database(m.database).Collection(collectionSysLog)

	// Build match stage with enabled charge points filter
	matchStage := bson.D{
		{"$match", bson.D{
			{"text", bson.D{{"$regex", "registered"}}},
			{"charge_point_id", bson.D{{"$in", enabledCPs}}},
		}},
	}

	pipeline := mongo.Pipeline{
		matchStage,
		// Sort by charge_point_id and timestamp descending
		{{"$sort", bson.D{
			{"charge_point_id", 1},
			{"timestamp", -1},
		}}},
		// Group by charge_point_id, take the first (most recent) event
		{{"$group", bson.D{
			{"_id", "$charge_point_id"},
			{"text", bson.D{{"$first", "$text"}}},
			{"timestamp", bson.D{{"$first", "$timestamp"}}},
		}}},
		// Sort by charge_point_id
		{{"$sort", bson.D{{"_id", 1}}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	defer cursor.Close(ctx)

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
	collection := m.client.Database(m.database).Collection(collectionChargePoints)

	filter := bson.D{{"is_enabled", true}}
	if chargePointId != "" {
		filter = append(filter, bson.E{Key: "charge_point_id", Value: chargePointId})
	}

	// Only fetch the charge_point_id field
	opts := options.Find().SetProjection(bson.D{{"charge_point_id", 1}})
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, m.findError(err)
	}
	defer cursor.Close(ctx)

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
