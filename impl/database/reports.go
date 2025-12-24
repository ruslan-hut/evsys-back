package database

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

// TotalsByMonth returns the total consumed watts, average watts, and count of transactions by month
func (m *MongoDB) TotalsByMonth(from, to time.Time, userGroup string) ([]interface{}, error) {
	ctx := context.Background()
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
func (m *MongoDB) TotalsByUsers(from, to time.Time, userGroup string) ([]interface{}, error) {
	ctx := context.Background()
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

func (m *MongoDB) TotalsByCharger(from, to time.Time, userGroup string) ([]interface{}, error) {
	ctx := context.Background()
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
