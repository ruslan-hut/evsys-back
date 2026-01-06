package database

import (
	"context"
	"errors"
	"evsys-back/config"
	"evsys-back/entity"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	collectionSysLog         = "sys_log"
	collectionBackLog        = "back_log"
	collectionPaymentLog     = "payment_log"
	collectionConfig         = "config"
	collectionUsers          = "users"
	collectionUserTags       = "user_tags"
	collectionChargePoints   = "charge_points"
	collectionConnectors     = "connectors"
	collectionTransactions   = "transactions"
	collectionMeterValues    = "meter_values"
	collectionInvites        = "invites"
	collectionPayment        = "payment"
	collectionPaymentMethods = "payment_methods"
	collectionPaymentOrders  = "payment_orders"
	collectionPaymentPlans   = "payment_plans"
	collectionLocations      = "locations"
)

type MongoDB struct {
	client           *mongo.Client
	database         string
	logRecordsNumber int64
}

func (m *MongoDB) findError(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil
	}
	return fmt.Errorf("mongodb find error: %w", err)
}

func NewMongoClient(conf *config.Config) (*MongoDB, error) {
	if !conf.Mongo.Enabled {
		return nil, nil
	}
	connectionUri := fmt.Sprintf("mongodb://%s:%s", conf.Mongo.Host, conf.Mongo.Port)
	clientOptions := options.Client().ApplyURI(connectionUri)
	if conf.Mongo.User != "" {
		clientOptions.SetAuth(options.Credential{
			Username:   conf.Mongo.User,
			Password:   conf.Mongo.Password,
			AuthSource: conf.Mongo.Database,
		})
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("connecting to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("pinging mongodb: %w", err)
	}

	return &MongoDB{
		client:           client,
		database:         conf.Mongo.Database,
		logRecordsNumber: conf.LogRecords,
	}, nil
}

func (m *MongoDB) Close() error {
	if m.client != nil {
		return m.client.Disconnect(context.Background())
	}
	return nil
}

func (m *MongoDB) read(ctx context.Context, table, dataType string) (interface{}, error) {
	var logMessages interface{}
	timeFieldName := "timestamp"

	switch dataType {
	case entity.FeatureMessageType:
		logMessages = []entity.FeatureMessage{}
	case entity.LogMessageType:
		logMessages = []entity.LogMessage{}
		timeFieldName = "time"
	default:
		return nil, fmt.Errorf("unknown data type: %s", dataType)
	}

	collection := m.client.Database(m.database).Collection(table)
	filter := bson.D{}
	opts := options.Find().SetSort(bson.D{{timeFieldName, -1}})
	if m.logRecordsNumber > 0 {
		opts.SetLimit(m.logRecordsNumber)
	}
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(ctx, &logMessages); err != nil {
		return nil, err
	}
	return logMessages, nil
}

func (m *MongoDB) ReadLog(ctx context.Context, logName string) (interface{}, error) {
	switch logName {
	case "sys":
		return m.read(ctx, collectionSysLog, entity.FeatureMessageType)
	case "back":
		return m.read(ctx, collectionBackLog, entity.LogMessageType)
	case "pay":
		return m.read(ctx, collectionPaymentLog, entity.LogMessageType)
	default:
		return nil, fmt.Errorf("unknown log name: %s", logName)
	}
}

// ReadLogAfter returns array of log messages in a normal , filtered by timeStart
func (m *MongoDB) ReadLogAfter(ctx context.Context, timeStart time.Time) ([]*entity.FeatureMessage, error) {
	collection := m.client.Database(m.database).Collection(collectionSysLog)
	var pipe []bson.M
	pipe = append(pipe, bson.M{"$match": bson.M{"timestamp": bson.M{"$gt": timeStart}}})
	pipe = append(pipe, bson.M{"$sort": bson.M{"timestamp": 1}})
	pipe = append(pipe, bson.M{"$limit": m.logRecordsNumber})
	cursor, err := collection.Aggregate(ctx, pipe)
	if err != nil {
		return nil, err
	}
	var result []*entity.FeatureMessage
	err = cursor.All(ctx, &result)
	if err != nil {
		return nil, m.findError(err)
	}
	return result, nil
}

func (m *MongoDB) GetConfig(ctx context.Context, name string) (interface{}, error) {
	collection := m.client.Database(m.database).Collection(collectionConfig)
	filter := bson.D{{"name", name}}
	var configData interface{}
	err := collection.FindOne(ctx, filter).Decode(&configData)
	if err != nil {
		return nil, m.findError(err)
	}
	return configData, nil
}

func (m *MongoDB) GetUser(ctx context.Context, username string) (*entity.User, error) {
	collection := m.client.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"username", username}}
	var userData entity.User
	err := collection.FindOne(ctx, filter).Decode(&userData)
	if err != nil {
		return nil, m.findError(err)
	}
	return &userData, nil
}

// GetUserInfo get user info: data of a user, merged with tags, payment methods, payment plans
func (m *MongoDB) GetUserInfo(ctx context.Context, _ int, username string) (*entity.UserInfo, error) {
	pipeline := mongo.Pipeline{
		{
			{"$match", bson.M{"username": username}},
			//	"$and": []bson.M{
			//		{"username": username},
			//		{"access_level": bson.M{"$lte": level}},
			//	},
			//}},
		},
		{{"$lookup", bson.M{
			"from":         collectionPaymentPlans,
			"localField":   "payment_plan",
			"foreignField": "plan_id",
			"as":           "payment_plans",
		}}},
		{{"$lookup", bson.M{
			"from":         collectionPaymentMethods,
			"localField":   "username",
			"foreignField": "user_name",
			"as":           "payment_methods",
		}}},
		{{"$lookup", bson.M{
			"from":         collectionUserTags,
			"localField":   "username",
			"foreignField": "username",
			"as":           "user_tags",
		}}},
		{{"$project", bson.M{
			//"payment_methods.identifier": 0,
			"payment_methods.user_id": 0,
			"user_tags.user_id":       0,
		}}},
	}
	collection := m.client.Database(m.database).Collection(collectionUsers)
	var info entity.UserInfo
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	if cursor.Next(ctx) {
		if err = cursor.Decode(&info); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("no user found")
	}

	return &info, nil
}

func (m *MongoDB) GetUsers(ctx context.Context) ([]*entity.User, error) {
	collection := m.client.Database(m.database).Collection(collectionUsers)
	filter := bson.D{}
	projection := bson.M{"password": 0, "token": 0}
	var users []*entity.User
	cursor, err := collection.Find(ctx, filter, options.Find().SetProjection(projection))
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (m *MongoDB) GetUserById(ctx context.Context, userId string) (*entity.User, error) {
	collection := m.client.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"user_id", userId}}
	var userData entity.User
	err := collection.FindOne(ctx, filter).Decode(&userData)
	if err != nil {
		return nil, m.findError(err)
	}
	return &userData, nil
}

func (m *MongoDB) GetUserTags(ctx context.Context, userId string) ([]entity.UserTag, error) {
	collection := m.client.Database(m.database).Collection(collectionUserTags)
	filter := bson.M{"$and": []bson.M{{"user_id": userId}, {"is_enabled": true}}}
	var userTags []entity.UserTag
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(ctx, &userTags); err != nil {
		return nil, err
	}
	return userTags, nil
}

func (m *MongoDB) GetUserTag(ctx context.Context, idTag string) (*entity.UserTag, error) {
	collection := m.client.Database(m.database).Collection(collectionUserTags)
	filter := bson.D{{"id_tag", idTag}}
	var userTag entity.UserTag
	if err := collection.FindOne(ctx, filter).Decode(&userTag); err != nil {
		return nil, m.findError(err)
	}
	return &userTag, nil
}

func (m *MongoDB) AddUserTag(ctx context.Context, userTag *entity.UserTag) error {
	t, _ := m.GetUserTag(ctx, userTag.IdTag)
	if t != nil {
		return fmt.Errorf("user tag %s already exists for user %s", t.IdTag, t.UserId)
	}
	collection := m.client.Database(m.database).Collection(collectionUserTags)
	_, err := collection.InsertOne(ctx, userTag)
	return err
}

// CheckUserTag check unique of idTag
func (m *MongoDB) CheckUserTag(ctx context.Context, idTag string) error {
	collection := m.client.Database(m.database).Collection(collectionUserTags)
	filter := bson.D{{"id_tag", idTag}}
	var userTag entity.UserTag
	err := collection.FindOne(ctx, filter).Decode(&userTag)
	if err == nil {
		return fmt.Errorf("user tag %s already exists", userTag.IdTag)
	}
	return nil
}

func (m *MongoDB) UpdateTagLastSeen(ctx context.Context, userTag *entity.UserTag) error {
	collection := m.client.Database(m.database).Collection(collectionUserTags)
	filter := bson.D{{"id_tag", userTag.IdTag}}
	update := bson.M{"$set": bson.D{
		{"last_seen", time.Now()},
	}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (m *MongoDB) GetAllUserTags(ctx context.Context) ([]*entity.UserTag, error) {
	collection := m.client.Database(m.database).Collection(collectionUserTags)
	var userTags []*entity.UserTag
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(ctx, &userTags); err != nil {
		return nil, err
	}
	return userTags, nil
}

func (m *MongoDB) GetUserTagByIdTag(ctx context.Context, idTag string) (*entity.UserTag, error) {
	return m.GetUserTag(ctx, idTag)
}

func (m *MongoDB) UpdateUserTag(ctx context.Context, userTag *entity.UserTag) error {
	collection := m.client.Database(m.database).Collection(collectionUserTags)
	filter := bson.D{{"id_tag", userTag.IdTag}}
	update := bson.M{"$set": bson.D{
		{"username", userTag.Username},
		{"user_id", userTag.UserId},
		{"source", userTag.Source},
		{"is_enabled", userTag.IsEnabled},
		{"local", userTag.Local},
		{"note", userTag.Note},
	}}
	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("tag not found")
	}
	return nil
}

func (m *MongoDB) DeleteUserTag(ctx context.Context, idTag string) error {
	collection := m.client.Database(m.database).Collection(collectionUserTags)
	filter := bson.D{{"id_tag", idTag}}
	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("tag not found")
	}
	return nil
}

func (m *MongoDB) UpdateLastSeen(ctx context.Context, user *entity.User) error {
	collection := m.client.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"username", user.Username}}
	update := bson.M{"$set": bson.D{
		{"last_seen", time.Now()},
		{"token", user.Token},
	}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (m *MongoDB) AddUser(ctx context.Context, user *entity.User) error {
	collection := m.client.Database(m.database).Collection(collectionUsers)
	_, err := collection.InsertOne(ctx, user)
	return err
}

func (m *MongoDB) UpdateUser(ctx context.Context, user *entity.User) error {
	collection := m.client.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"username", user.Username}}
	update := bson.M{"$set": bson.D{
		{"name", user.Name},
		{"email", user.Email},
		{"role", user.Role},
		{"access_level", user.AccessLevel},
		{"payment_plan", user.PaymentPlan},
		{"password", user.Password},
	}}
	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (m *MongoDB) DeleteUser(ctx context.Context, username string) error {
	collection := m.client.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"username", username}}
	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// CheckUsername check unique username
func (m *MongoDB) CheckUsername(ctx context.Context, username string) error {
	collection := m.client.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"username", username}}
	var userData entity.User
	err := collection.FindOne(ctx, filter).Decode(&userData)
	if err == nil {
		return fmt.Errorf("username %s already exists", username)
	}
	return nil
}

func (m *MongoDB) CheckToken(ctx context.Context, token string) (*entity.User, error) {
	collection := m.client.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"token", token}}
	var userData entity.User
	err := collection.FindOne(ctx, filter).Decode(&userData)
	if err != nil {
		return nil, err
	}
	return &userData, nil
}

func (m *MongoDB) GetChargePoints(ctx context.Context, level int, searchTerm string) ([]*entity.ChargePoint, error) {
	var searchRegex = bson.M{"$regex": primitive.Regex{Pattern: ".*" + searchTerm + ".*", Options: "i"}}

	pipeline := mongo.Pipeline{
		{
			{"$match", bson.M{
				"$and": []bson.M{
					{"$or": []bson.M{
						{"description": searchRegex},
						{"title": searchRegex},
						{"address": searchRegex},
					}},
					{"access_level": bson.M{"$lte": level}},
				},
			}},
		},
		{{"$lookup", bson.M{
			"from":         collectionConnectors,
			"localField":   "charge_point_id",
			"foreignField": "charge_point_id",
			"as":           "Connectors",
		}}},
		bson.D{{"$sort", bson.D{{"charge_point_id", 1}}}},
	}

	collection := m.client.Database(m.database).Collection(collectionChargePoints)
	var chargePoints []*entity.ChargePoint
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(ctx, &chargePoints); err != nil {
		return nil, err
	}

	return chargePoints, nil
}

func (m *MongoDB) GetChargePoint(ctx context.Context, level int, id string) (*entity.ChargePoint, error) {
	collection := m.client.Database(m.database).Collection(collectionChargePoints)
	filter := bson.M{"$and": []bson.M{{"charge_point_id": id}, {"access_level": bson.M{"$lte": level}}}}

	pipeline := mongo.Pipeline{
		bson.D{{"$match", filter}}, // Match the charge point
		bson.D{{"$lookup", bson.D{ // Lookup to join with connectors collection
			{"from", collectionConnectors},      // The collection to join
			{"localField", "charge_point_id"},   // Field from charge_points collection
			{"foreignField", "charge_point_id"}, // Field from connectors collection
			{"as", "Connectors"},                // The array field to add in chargePoint
		}}},
	}

	var chargePoint entity.ChargePoint
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	if cursor.Next(ctx) {
		if err = cursor.Decode(&chargePoint); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("no charge point found")
	}
	return &chargePoint, nil
}

func (m *MongoDB) UpdateChargePoint(ctx context.Context, level int, chargePoint *entity.ChargePoint) error {
	collection := m.client.Database(m.database).Collection(collectionChargePoints)
	filter := bson.M{"$and": []bson.M{
		{"charge_point_id": chargePoint.Id},
		{"access_level": bson.M{"$lte": level}},
	}}
	update := bson.M{"$set": bson.D{
		{"title", chargePoint.Title},
		{"description", chargePoint.Description},
		{"address", chargePoint.Address},
		{"access_type", chargePoint.AccessType},
		{"access_level", chargePoint.AccessLevel},
		{"location.latitude", chargePoint.Location.Latitude},
		{"location.longitude", chargePoint.Location.Longitude},
	}}
	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("update charge point: %v", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("no charge point found")
	}

	// update connectors data
	collection = m.client.Database(m.database).Collection(collectionConnectors)
	for _, connector := range chargePoint.Connectors {
		filter = bson.M{"charge_point_id": chargePoint.Id, "connector_id": connector.Id}
		update = bson.M{"$set": bson.D{
			{"type", connector.Type},
			{"power", connector.Power},
			{"connector_id_name", connector.IdName},
		}}
		_, err = collection.UpdateOne(ctx, filter, update)
	}
	return err
}

func (m *MongoDB) GetConnector(chargePointId string, connectorId int) (*entity.Connector, error) {
	ctx := context.Background()
	collection := m.client.Database(m.database).Collection(collectionConnectors)
	filter := bson.D{{"charge_point_id", chargePointId}, {"connector_id", connectorId}}
	var connector entity.Connector
	if err := collection.FindOne(ctx, filter).Decode(&connector); err != nil {
		return nil, m.findError(err)
	}
	return &connector, nil
}

func (m *MongoDB) getTransactionState(ctx context.Context, userId string, level int, transaction *entity.Transaction) (*entity.ChargeState, error) {
	var chargeState entity.ChargeState

	chargePoint, err := m.GetChargePoint(ctx, level, transaction.ChargePointId)
	if err != nil {
		return nil, fmt.Errorf("get charge point: %v", err)
	}
	if chargePoint == nil {
		return nil, fmt.Errorf("charge point not found")
	}
	connector, err := chargePoint.GetConnector(transaction.ConnectorId)
	if err != nil {
		return nil, fmt.Errorf("get connector: %v", err)
	}

	consumed := 0
	if transaction.MeterStop > transaction.MeterStart {
		consumed = transaction.MeterStop - transaction.MeterStart
	}
	price := transaction.PaymentAmount
	powerRate := 0
	meterValues := transaction.MeterValues
	duration := int(transaction.TimeStop.Sub(transaction.TimeStart).Seconds())

	if !transaction.IsFinished {

		lastMeter, _ := m.GetLastMeterValue(ctx, transaction.TransactionId)
		if lastMeter != nil {
			if lastMeter.Value > transaction.MeterStart {
				consumed = lastMeter.Value - transaction.MeterStart
			}
			price = lastMeter.Price
			powerRate = lastMeter.PowerRate
		}

		meterValues, err = m.GetMeterValues(ctx, transaction.TransactionId, transaction.TimeStart)

		if len(meterValues) == 0 && lastMeter != nil {
			meterValues = append(meterValues, *lastMeter)
		}

		duration = int(time.Since(transaction.TimeStart).Seconds())

	}

	// current user can stop own transaction
	canStop := false
	if !transaction.IsFinished && transaction.UserTag != nil {
		canStop = transaction.UserTag.UserId == userId
	}

	chargeState = entity.ChargeState{
		TransactionId:      transaction.TransactionId,
		ConnectorId:        transaction.ConnectorId,
		Connector:          connector,
		ChargePointId:      transaction.ChargePointId,
		ChargePointTitle:   chargePoint.Title,
		ChargePointAddress: chargePoint.Address,
		TimeStarted:        transaction.TimeStart,
		Duration:           duration,
		MeterStart:         transaction.MeterStart,
		Consumed:           consumed,
		PowerRate:          powerRate,
		Price:              price,
		PaymentBilled:      transaction.PaymentBilled,
		Status:             connector.Status,
		IsCharging:         transaction.IsFinished == false,
		CanStop:            canStop,
		MeterValues:        meterValues,
		IdTag:              transaction.IdTag,
		PaymentPlan:        transaction.Plan,
		PaymentMethod:      transaction.PaymentMethod,
		PaymentOrders:      transaction.PaymentOrders,
	}

	return &chargeState, nil
}

func (m *MongoDB) GetTransactionState(ctx context.Context, userId string, level int, id int) (*entity.ChargeState, error) {
	transaction, err := m.GetTransaction(ctx, id)
	if err != nil {
		return nil, err
	}
	chargeState, err := m.getTransactionState(ctx, userId, level, transaction)
	if err != nil {
		return nil, err
	}
	return chargeState, nil
}

func (m *MongoDB) GetTransaction(ctx context.Context, id int) (*entity.Transaction, error) {
	collection := m.client.Database(m.database).Collection(collectionTransactions)
	filter := bson.D{{"transaction_id", id}}
	var transaction entity.Transaction
	if err := collection.FindOne(ctx, filter).Decode(&transaction); err != nil {
		return nil, m.findError(err)
	}
	return &transaction, nil
}

// UpdateTransaction update transaction billed data
func (m *MongoDB) UpdateTransaction(transaction *entity.Transaction) error {
	ctx := context.Background()
	collection := m.client.Database(m.database).Collection(collectionTransactions)
	filter := bson.D{{"transaction_id", transaction.TransactionId}}
	update := bson.D{
		{"$set", bson.D{
			{"payment_order", transaction.PaymentOrder},
			{"payment_billed", transaction.PaymentBilled},
		}},
	}
	if _, err := collection.UpdateOne(ctx, filter, update); err != nil {
		return err
	}
	return nil
}

func (m *MongoDB) GetActiveTransactions(ctx context.Context, userId string) ([]*entity.ChargeState, error) {
	user, err := m.GetUserById(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("get user: %v", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	tags, err := m.GetUserTags(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("get user tags: %v", err)
	}
	if len(tags) == 0 {
		return nil, nil
	}

	var idTags []string
	for _, tag := range tags {
		idTags = append(idTags, strings.ToUpper(tag.IdTag))
	}

	collection := m.client.Database(m.database).Collection(collectionTransactions)
	filter := bson.D{
		{"id_tag", bson.D{{"$in", idTags}}},
		{"is_finished", false},
	}
	var transactions []entity.Transaction
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(ctx, &transactions); err != nil {
		return nil, err
	}
	if len(transactions) == 0 {
		return nil, nil
	}

	var chargeStates []*entity.ChargeState
	for _, transaction := range transactions {
		chargeState, _ := m.getTransactionState(ctx, userId, user.AccessLevel, &transaction)
		if chargeState != nil {
			chargeStates = append(chargeStates, chargeState)
		}
	}
	return chargeStates, nil
}

// GetTransactionsToBill get list of transactions to bill
func (m *MongoDB) GetTransactionsToBill(ctx context.Context, userId string) ([]*entity.Transaction, error) {
	tags, err := m.GetUserTags(ctx, userId)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, nil
	}

	var idTags []string
	for _, tag := range tags {
		idTags = append(idTags, strings.ToUpper(tag.IdTag))
	}

	collection := m.client.Database(m.database).Collection(collectionTransactions)
	filter := bson.D{
		{"id_tag", bson.D{{"$in", idTags}}},
		{"is_finished", true},
		{"$expr", bson.D{{"$lt", bson.A{"$payment_billed", "$payment_amount"}}}},
	}
	var transactions []*entity.Transaction
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(ctx, &transactions); err != nil {
		return nil, err
	}
	return transactions, nil
}

// GetTransactions gets list of a user's transactions; if period is empty, returns last 100 transactions
func (m *MongoDB) GetTransactions(ctx context.Context, userId string, period string) ([]*entity.Transaction, error) {
	tags, err := m.GetUserTags(ctx, userId)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, nil
	}

	var idTags []string
	for _, tag := range tags {
		idTags = append(idTags, strings.ToUpper(tag.IdTag))
	}

	collection := m.client.Database(m.database).Collection(collectionTransactions)

	var transactions []*entity.Transaction
	var cursor *mongo.Cursor

	time1, time2, err := parsePeriod(period)
	if err != nil {
		filter := bson.D{
			{"id_tag", bson.D{{"$in", idTags}}},
			{"is_finished", true},
		}
		cursor, err = collection.Find(ctx, filter, options.Find().SetLimit(int64(100)).SetSkip(int64(0)).SetSort(bson.D{{"time_start", -1}}))
	} else {
		filter := bson.D{
			{"id_tag", bson.D{{"$in", idTags}}},
			{"is_finished", true},
			{"time_stop", bson.D{{"$gte", time1}}},
			{"time_stop", bson.D{{"$lte", time2}}},
		}
		cursor, err = collection.Find(ctx, filter, options.Find().SetSort(bson.D{{"time_start", -1}}))
	}

	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(ctx, &transactions); err != nil {
		return nil, err
	}
	return transactions, nil
}

// GetFilteredTransactions returns transactions filtered by the provided criteria
func (m *MongoDB) GetFilteredTransactions(ctx context.Context, filter *entity.TransactionFilter) ([]*entity.Transaction, error) {
	collection := m.client.Database(m.database).Collection(collectionTransactions)

	// Build dynamic filter
	mongoFilter := bson.D{}

	// Filter by username: lookup user's id_tags first
	if filter.Username != "" {
		tagsCollection := m.client.Database(m.database).Collection(collectionUserTags)
		tagFilter := bson.D{{"username", filter.Username}}
		cursor, err := tagsCollection.Find(ctx, tagFilter)
		if err != nil {
			return nil, m.findError(err)
		}
		var userTags []*entity.UserTag
		if err = cursor.All(ctx, &userTags); err != nil {
			return nil, err
		}
		if len(userTags) == 0 {
			// No tags found for this user, return empty result
			return []*entity.Transaction{}, nil
		}
		var idTags []string
		for _, tag := range userTags {
			idTags = append(idTags, strings.ToUpper(tag.IdTag))
		}
		mongoFilter = append(mongoFilter, bson.E{"id_tag", bson.D{{"$in", idTags}}})
	}

	// Filter by id_tag
	if filter.IdTag != "" {
		mongoFilter = append(mongoFilter, bson.E{"id_tag", strings.ToUpper(filter.IdTag)})
	}

	// Filter by charge_point_id
	if filter.ChargePointId != "" {
		mongoFilter = append(mongoFilter, bson.E{"charge_point_id", filter.ChargePointId})
	}

	// Filter by date range
	if filter.From != nil {
		mongoFilter = append(mongoFilter, bson.E{"time_start", bson.D{{"$gte", *filter.From}}})
	}
	if filter.To != nil {
		mongoFilter = append(mongoFilter, bson.E{"time_start", bson.D{{"$lte", *filter.To}}})
	}

	// Only finished transactions
	mongoFilter = append(mongoFilter, bson.E{"is_finished", true})

	// Sort by time_start descending (newest first)
	opts := options.Find().SetSort(bson.D{{"time_start", -1}})

	cursor, err := collection.Find(ctx, mongoFilter, opts)
	if err != nil {
		return nil, m.findError(err)
	}

	var transactions []*entity.Transaction
	if err = cursor.All(ctx, &transactions); err != nil {
		return nil, err
	}

	return transactions, nil
}

func (m *MongoDB) GetTransactionByTag(ctx context.Context, idTag string, timeStart time.Time) (*entity.Transaction, error) {
	collection := m.client.Database(m.database).Collection(collectionTransactions)
	filter := bson.D{
		{"id_tag", strings.ToUpper(idTag)},
		{"time_start", bson.D{{"$gte", timeStart}}},
	}
	var transaction entity.Transaction
	if err := collection.FindOne(ctx, filter).Decode(&transaction); err != nil {
		return nil, m.findError(err)
	}
	return &transaction, nil
}

func (m *MongoDB) GetLastMeterValue(ctx context.Context, transactionId int) (*entity.TransactionMeter, error) {
	collection := m.client.Database(m.database).Collection(collectionMeterValues)
	filter := bson.D{
		{"transaction_id", transactionId},
		{"measurand", "Energy.Active.Import.Register"},
	}
	var value entity.TransactionMeter
	if err := collection.FindOne(ctx, filter, options.FindOne().SetSort(bson.D{{"time", -1}})).Decode(&value); err != nil {
		return nil, m.findError(err)
	}
	return &value, nil
}

func (m *MongoDB) GetMeterValues(ctx context.Context, transactionId int, from time.Time) ([]entity.TransactionMeter, error) {
	filter := bson.D{
		{"transaction_id", transactionId},
		{"measurand", "Energy.Active.Import.Register"},
		{"time", bson.D{{"$gte", from}}},
	}
	collection := m.client.Database(m.database).Collection(collectionMeterValues)
	opts := options.Find().SetSort(bson.D{{"time", 1}})
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var meterValues []entity.TransactionMeter
	if err = cursor.All(ctx, &meterValues); err != nil {
		return nil, m.findError(err)
	}
	return meterValues, nil
}

func (m *MongoDB) GetRecentUserChargePoints(ctx context.Context, userId string) ([]*entity.ChargePoint, error) {
	// Get last 3 transactions for user's tags
	tags, err := m.GetUserTags(ctx, userId)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, nil
	}

	var idTags []string
	for _, tag := range tags {
		idTags = append(idTags, strings.ToUpper(tag.IdTag))
	}

	// Get transactions
	collection := m.client.Database(m.database).Collection(collectionTransactions)
	filter := bson.D{
		{"id_tag", bson.D{{"$in", idTags}}},
		{"is_finished", true},
	}
	opts := options.Find().SetSort(bson.D{{"time_start", -1}}).SetLimit(3)
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, m.findError(err)
	}
	var transactions []entity.Transaction
	if err = cursor.All(ctx, &transactions); err != nil {
		return nil, err
	}

	// Extract charge point IDs
	var chargePointIds []string
	for _, t := range transactions {
		chargePointIds = append(chargePointIds, t.ChargePointId)
	}

	// Get charge points
	if len(chargePointIds) == 0 {
		return m.GetChargePoints(ctx, 0, "")
	}

	pipeline := mongo.Pipeline{
		{{"$match", bson.D{{"charge_point_id", bson.D{{"$in", chargePointIds}}}}}},
		{{"$lookup", bson.M{
			"from":         collectionConnectors,
			"localField":   "charge_point_id",
			"foreignField": "charge_point_id",
			"as":           "Connectors",
		}}},
	}

	collection = m.client.Database(m.database).Collection(collectionChargePoints)
	var chargePoints []*entity.ChargePoint
	cursor, err = collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(ctx, &chargePoints); err != nil {
		return nil, err
	}

	return chargePoints, nil
}

func (m *MongoDB) AddInviteCode(ctx context.Context, invite *entity.Invite) error {
	collection := m.client.Database(m.database).Collection(collectionInvites)
	_, err := collection.InsertOne(ctx, invite)
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoDB) CheckInviteCode(ctx context.Context, code string) (bool, error) {
	collection := m.client.Database(m.database).Collection(collectionInvites)
	filter := bson.D{{"code", code}}
	var inviteCode entity.Invite
	if err := collection.FindOne(ctx, filter).Decode(&inviteCode); err != nil {
		return false, err
	}
	return true, nil
}

func (m *MongoDB) DeleteInviteCode(ctx context.Context, code string) error {
	collection := m.client.Database(m.database).Collection(collectionInvites)
	filter := bson.D{{"code", code}}
	_, err := collection.DeleteOne(ctx, filter)
	return err
}

func (m *MongoDB) SavePaymentResult(paymentParameters *entity.PaymentParameters) error {
	ctx := context.Background()
	collection := m.client.Database(m.database).Collection(collectionPayment)
	_, err := collection.InsertOne(ctx, paymentParameters)
	return err
}

// GetPaymentParameters get payment parameters by order id
func (m *MongoDB) GetPaymentParameters(orderId string) (*entity.PaymentParameters, error) {
	ctx := context.Background()
	collection := m.client.Database(m.database).Collection(collectionPayment)
	filter := bson.D{{"order", orderId}}
	var paymentParameters entity.PaymentParameters
	if err := collection.FindOne(ctx, filter).Decode(&paymentParameters); err != nil {
		return nil, m.findError(err)
	}
	return &paymentParameters, nil
}

func (m *MongoDB) SavePaymentMethod(ctx context.Context, paymentMethod *entity.PaymentMethod) error {
	saved, _ := m.GetPaymentMethod(ctx, paymentMethod.Identifier, paymentMethod.UserId)
	if saved != nil {
		return fmt.Errorf("payment method with identifier %s... already exists", paymentMethod.Identifier[0:10])
	}

	collection := m.client.Database(m.database).Collection(collectionPaymentMethods)
	_, err := collection.InsertOne(ctx, paymentMethod)
	if err != nil {
		return err
	}
	return nil
}

// UpdatePaymentMethod update user-editable fields for payment method
func (m *MongoDB) UpdatePaymentMethod(ctx context.Context, paymentMethod *entity.PaymentMethod) error {
	collection := m.client.Database(m.database).Collection(collectionPaymentMethods)

	// if current method is default, then set all other methods to not default
	if paymentMethod.IsDefault {
		filter := bson.D{{"user_id", paymentMethod.UserId}}
		update := bson.M{"$set": bson.D{{"is_default", false}}}
		_, err := collection.UpdateMany(ctx, filter, update)
		if err != nil {
			return err
		}
	}

	filter := bson.D{{"identifier", paymentMethod.Identifier}, {"user_id", paymentMethod.UserId}}
	update := bson.M{"$set": bson.D{
		{"description", paymentMethod.Description},
		{"is_default", paymentMethod.IsDefault},
	}}
	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

// DeletePaymentMethod delete payment method by identifier
func (m *MongoDB) DeletePaymentMethod(ctx context.Context, paymentMethod *entity.PaymentMethod) error {
	isDefault := paymentMethod.IsDefault

	collection := m.client.Database(m.database).Collection(collectionPaymentMethods)
	filter := bson.D{{"identifier", paymentMethod.Identifier}, {"user_id", paymentMethod.UserId}}
	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("payment method not found")
	}

	// if we deleted default method, then set first method to default
	if isDefault {
		var method entity.PaymentMethod
		filter = bson.D{{"user_id", paymentMethod.UserId}}
		err = collection.FindOne(ctx, filter).Decode(&method)
		if err == nil {
			filter = bson.D{{"identifier", method.Identifier}, {"user_id", method.UserId}}
			update := bson.M{"$set": bson.D{
				{"is_default", true},
			}}
			_, _ = collection.UpdateOne(ctx, filter, update)
		}
	}
	return nil
}

// GetPaymentMethods get payment methods by user id
func (m *MongoDB) GetPaymentMethods(ctx context.Context, userId string) ([]*entity.PaymentMethod, error) {
	collection := m.client.Database(m.database).Collection(collectionPaymentMethods)
	filter := bson.D{{"user_id", userId}}
	var paymentMethods []*entity.PaymentMethod
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(ctx, &paymentMethods); err != nil {
		return nil, err
	}
	return paymentMethods, nil
}

// GetPaymentMethod get payment method by identifier
func (m *MongoDB) GetPaymentMethod(ctx context.Context, identifier, userId string) (*entity.PaymentMethod, error) {
	collection := m.client.Database(m.database).Collection(collectionPaymentMethods)
	filter := bson.D{{"identifier", identifier}, {"user_id", userId}}
	var paymentMethod entity.PaymentMethod
	if err := collection.FindOne(ctx, filter).Decode(&paymentMethod); err != nil {
		return nil, m.findError(err)
	}
	return &paymentMethod, nil
}

func (m *MongoDB) GetLastOrder(ctx context.Context) (*entity.PaymentOrder, error) {
	collection := m.client.Database(m.database).Collection(collectionPaymentOrders)
	filter := bson.D{}
	var order entity.PaymentOrder
	if err := collection.FindOne(ctx, filter, options.FindOne().SetSort(bson.D{{"time_opened", -1}})).Decode(&order); err != nil {
		return nil, m.findError(err)
	}
	return &order, nil
}

// GetPaymentOrderByTransaction get payment order by transaction id
func (m *MongoDB) GetPaymentOrderByTransaction(ctx context.Context, transactionId int) (*entity.PaymentOrder, error) {
	collection := m.client.Database(m.database).Collection(collectionPaymentOrders)
	filter := bson.D{{"transaction_id", transactionId}, {"is_completed", false}}
	var order entity.PaymentOrder
	if err := collection.FindOne(ctx, filter).Decode(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (m *MongoDB) GetPaymentOrder(id int) (*entity.PaymentOrder, error) {
	ctx := context.Background()
	collection := m.client.Database(m.database).Collection(collectionPaymentOrders)
	filter := bson.D{{"order", id}}
	var order entity.PaymentOrder
	if err := collection.FindOne(ctx, filter).Decode(&order); err != nil {
		return nil, m.findError(err)
	}
	return &order, nil
}

// SavePaymentOrder insert or update order
func (m *MongoDB) SavePaymentOrder(ctx context.Context, order *entity.PaymentOrder) error {
	filter := bson.D{{"order", order.Order}}
	set := bson.M{"$set": order}
	collection := m.client.Database(m.database).Collection(collectionPaymentOrders)
	_, err := collection.UpdateOne(ctx, filter, set, options.Update().SetUpsert(true))
	if err != nil {
		return err
	}
	return nil
}

// Helper function to convert a string of milliseconds to a time.Time value
func millisToTime(millis string) (time.Time, error) {
	// Convert the string to an int64
	millisInt, err := strconv.ParseInt(millis, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	// Convert the int64 to a time.Time value
	return time.Unix(0, millisInt*int64(time.Millisecond)), nil
}

func parsePeriod(period string) (time1, time2 time.Time, err error) {
	parts := strings.Split(period, "-")
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid period format")
	}
	time1, err = millisToTime(parts[0])
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	time2, err = millisToTime(parts[1])
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return time1, time2, nil
}

// GetLocations get all locations with all nested charge points and connectors
func (m *MongoDB) GetLocations(ctx context.Context) ([]*entity.Location, error) {
	pipeline := bson.A{
		bson.D{
			{"$match", bson.M{
				"roaming": true,
			}},
		},
		bson.D{
			{"$lookup",
				bson.D{
					{"from", collectionChargePoints},
					{"localField", "id"},
					{"foreignField", "location_id"},
					{"as", "charge_points"},
				},
			},
		},
		bson.D{{"$unwind", bson.D{{"path", "$charge_points"}}}},
		bson.D{
			{"$lookup",
				bson.D{
					{"from", collectionConnectors},
					{"localField", collectionChargePoints + ".charge_point_id"},
					{"foreignField", "charge_point_id"},
					{"as", "charge_points.connectors"},
				},
			},
		},
		bson.D{
			{"$group",
				bson.D{
					{"_id", "$id"},
					{"root", bson.D{{"$mergeObjects", "$$ROOT"}}},
					{"charge_points", bson.D{{"$push", "$charge_points"}}},
				},
			},
		},
		bson.D{
			{"$replaceRoot",
				bson.D{
					{"newRoot",
						bson.D{
							{"$mergeObjects",
								bson.A{
									"$root",
									bson.D{{"charge_points", "$charge_points"}},
								},
							},
						},
					},
				},
			},
		},
	}
	collection := m.client.Database(m.database).Collection(collectionLocations)
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	var locations []*entity.Location
	if err = cursor.All(ctx, &locations); err != nil {
		return nil, err
	}
	return locations, nil
}
