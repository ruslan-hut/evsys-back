package database

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

// TotalsByMonth returns the total consumed watts, average watts, and count of transactions by month
func (m *MongoDB) TotalsByMonth(from, to time.Time, userGroup string) ([]interface{}, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionTransactions)

	pipeline := mongo.Pipeline{
		// Stage 1: Lookup user tags by `id_tag`
		{{"$lookup", bson.D{
			{"from", collectionUserTags},
			{"localField", "id_tag"},
			{"foreignField", "id_tag"},
			{"as", "user_tag_info"},
		}}},
		// Stage 2: Unwind to de-nest user_tag_info array
		{{"$unwind", "$user_tag_info"}},
		// Stage 3: Lookup users by `user_id` obtained from `user_tag_info`
		{{"$lookup", bson.D{
			{"from", collectionUsers},
			{"localField", "user_tag_info.user_id"},
			{"foreignField", "user_id"},
			{"as", "user_info"},
		}}},
		// Stage 4: Unwind to de-nest user_info array
		{{"$unwind", "$user_info"}},
		// Stage 5: Filter transactions by a specific user group
		{{"$match", bson.D{
			//{"user_info.group", userGroup},
			{"transaction_stop", bson.D{
				{"$gte", from},
				{"$lte", to},
			}},
		}}},
		// Stage 6: Calculate consumed watts and group by year and month
		{{"$addFields", bson.D{
			{"consumed_watts", bson.D{
				{"$subtract", bson.A{"$meter_stop", "$meter_start"}},
			}},
		}}},
		{{"$group", bson.D{
			{"_id", bson.D{
				{"year", bson.D{{"$year", "$transaction_stop"}}},
				{"month", bson.D{{"$month", "$transaction_stop"}}},
			}},
			{"totalConsumedWatts", bson.D{{"$sum", "$consumed_watts"}}},
			{"avgWatts", bson.D{{"$avg", "$consumed_watts"}}},
			{"count", bson.D{{"$sum", 1}}},
		}}},
		// Stage 7: Sort by year and month
		{{"$sort", bson.D{
			{"_id.year", 1},
			{"_id.month", 1},
		}}},
		// (Optional) Stage 8: Reshape the output if needed
		{{"$project", bson.D{
			{"_id", 0},
			{"year", "$_id.year"},
			{"month", "$_id.month"},
			{"totalConsumedWatts", 1},
			{"avgWatts", 1},
			{"count", 1},
		}}},
	}

	cursor, err := collection.Aggregate(m.ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	var lines []interface{}
	if err = cursor.All(m.ctx, &lines); err != nil {
		return nil, err
	}
	//result := make([]interface{}, len(lines))
	//for i, v := range lines {
	//	result[i] = v
	//}
	return lines, err
}
