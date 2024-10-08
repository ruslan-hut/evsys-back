package database

import (
	"context"
	"errors"
	"evsys-back/config"
	"evsys-back/entity"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"strconv"
	"strings"
	"time"
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
	ctx              context.Context
	clientOptions    *options.ClientOptions
	database         string
	logRecordsNumber int64
}

func (m *MongoDB) connect() (*mongo.Client, error) {
	connection, err := mongo.Connect(m.ctx, m.clientOptions)
	if err != nil {
		return nil, err
	}
	return connection, nil
}

func (m *MongoDB) disconnect(connection *mongo.Client) {
	err := connection.Disconnect(m.ctx)
	if err != nil {
		log.Println("mongodb disconnect error", err)
	}
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
	client := &MongoDB{
		ctx:              context.Background(),
		clientOptions:    clientOptions,
		database:         conf.Mongo.Database,
		logRecordsNumber: conf.LogRecords,
	}
	return client, nil
}

func (m *MongoDB) read(table, dataType string) (interface{}, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

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

	collection := connection.Database(m.database).Collection(table)
	filter := bson.D{}
	opts := options.Find().SetSort(bson.D{{timeFieldName, -1}})
	if m.logRecordsNumber > 0 {
		opts.SetLimit(m.logRecordsNumber)
	}
	cursor, err := collection.Find(m.ctx, filter, opts)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(m.ctx, &logMessages); err != nil {
		return nil, err
	}
	return logMessages, nil
}

func (m *MongoDB) ReadLog(logName string) (interface{}, error) {
	switch logName {
	case "sys":
		return m.read(collectionSysLog, entity.FeatureMessageType)
	case "back":
		return m.read(collectionBackLog, entity.LogMessageType)
	case "pay":
		return m.read(collectionPaymentLog, entity.LogMessageType)
	default:
		return nil, fmt.Errorf("unknown log name: %s", logName)
	}
}

// ReadLogAfter returns array of log messages in a normal , filtered by timeStart
func (m *MongoDB) ReadLogAfter(timeStart time.Time) ([]*entity.FeatureMessage, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)
	collection := connection.Database(m.database).Collection(collectionSysLog)
	var pipe []bson.M
	pipe = append(pipe, bson.M{"$match": bson.M{"timestamp": bson.M{"$gt": timeStart}}})
	pipe = append(pipe, bson.M{"$sort": bson.M{"timestamp": 1}})
	pipe = append(pipe, bson.M{"$limit": m.logRecordsNumber})
	cursor, err := collection.Aggregate(m.ctx, pipe)
	if err != nil {
		return nil, err
	}
	var result []*entity.FeatureMessage
	err = cursor.All(m.ctx, &result)
	if err != nil {
		return nil, m.findError(err)
	}
	return result, nil
}

func (m *MongoDB) GetConfig(name string) (interface{}, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionConfig)
	filter := bson.D{{"name", name}}
	var configData interface{}
	err = collection.FindOne(m.ctx, filter).Decode(&configData)
	if err != nil {
		return nil, m.findError(err)
	}
	return configData, nil
}

func (m *MongoDB) GetUser(username string) (*entity.User, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)
	collection := connection.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"username", username}}
	var userData entity.User
	err = collection.FindOne(m.ctx, filter).Decode(&userData)
	if err != nil {
		return nil, m.findError(err)
	}
	return &userData, nil
}

// GetUserInfo get user info: data of a user, merged with tags, payment methods, payment plans
func (m *MongoDB) GetUserInfo(_ int, username string) (*entity.UserInfo, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

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
	collection := connection.Database(m.database).Collection(collectionUsers)
	var info entity.UserInfo
	cursor, err := collection.Aggregate(m.ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	if cursor.Next(m.ctx) {
		if err = cursor.Decode(&info); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("no user found")
	}

	return &info, nil
}

func (m *MongoDB) GetUsers() ([]*entity.User, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)
	collection := connection.Database(m.database).Collection(collectionUsers)
	filter := bson.D{}
	projection := bson.M{"password": 0, "token": 0}
	var users []*entity.User
	cursor, err := collection.Find(m.ctx, filter, options.Find().SetProjection(projection))
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(m.ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (m *MongoDB) GetUserById(userId string) (*entity.User, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"user_id", userId}}
	var userData entity.User
	err = collection.FindOne(m.ctx, filter).Decode(&userData)
	if err != nil {
		return nil, m.findError(err)
	}
	return &userData, nil
}

func (m *MongoDB) GetUserTags(userId string) ([]entity.UserTag, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUserTags)
	filter := bson.M{"$and": []bson.M{{"user_id": userId}, {"is_enabled": true}}}
	var userTags []entity.UserTag
	cursor, err := collection.Find(m.ctx, filter)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(m.ctx, &userTags); err != nil {
		return nil, err
	}
	return userTags, nil
}

func (m *MongoDB) GetUserTag(idTag string) (*entity.UserTag, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUserTags)
	filter := bson.D{{"id_tag", idTag}}
	var userTag entity.UserTag
	if err = collection.FindOne(m.ctx, filter).Decode(&userTag); err != nil {
		return nil, m.findError(err)
	}
	return &userTag, nil
}

func (m *MongoDB) AddUserTag(userTag *entity.UserTag) error {
	t, err := m.GetUserTag(userTag.IdTag)
	if t != nil {
		return fmt.Errorf("user tag %s already exists for user %s", t.IdTag, t.UserId)
	}
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUserTags)
	_, err = collection.InsertOne(m.ctx, userTag)
	return err
}

// CheckUserTag check unique of idTag
func (m *MongoDB) CheckUserTag(idTag string) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUserTags)
	filter := bson.D{{"id_tag", idTag}}
	var userTag entity.UserTag
	err = collection.FindOne(m.ctx, filter).Decode(&userTag)
	if err == nil {
		return fmt.Errorf("user tag %s already exists", userTag.IdTag)
	}
	return nil
}

func (m *MongoDB) UpdateTagLastSeen(userTag *entity.UserTag) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUserTags)
	filter := bson.D{{"id_tag", userTag.IdTag}}
	update := bson.M{"$set": bson.D{
		{"last_seen", time.Now()},
	}}
	_, err = collection.UpdateOne(m.ctx, filter, update)
	return err
}

func (m *MongoDB) UpdateLastSeen(user *entity.User) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"username", user.Username}}
	update := bson.M{"$set": bson.D{
		{"last_seen", time.Now()},
		{"token", user.Token},
	}}
	_, err = collection.UpdateOne(m.ctx, filter, update)
	return err
}

func (m *MongoDB) AddUser(user *entity.User) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUsers)
	_, err = collection.InsertOne(m.ctx, user)
	return err
}

// CheckUsername check unique username
func (m *MongoDB) CheckUsername(username string) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"username", username}}
	var userData entity.User
	err = collection.FindOne(m.ctx, filter).Decode(&userData)
	if err == nil {
		return fmt.Errorf("username %s already exists", username)
	}
	return nil
}

func (m *MongoDB) CheckToken(token string) (*entity.User, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"token", token}}
	var userData entity.User
	err = collection.FindOne(m.ctx, filter).Decode(&userData)
	if err != nil {
		return nil, err
	}
	return &userData, nil
}

func (m *MongoDB) GetChargePoints(level int, searchTerm string) ([]*entity.ChargePoint, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

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

	collection := connection.Database(m.database).Collection(collectionChargePoints)
	var chargePoints []*entity.ChargePoint
	cursor, err := collection.Aggregate(m.ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(m.ctx, &chargePoints); err != nil {
		return nil, err
	}

	return chargePoints, nil
}

func (m *MongoDB) GetChargePoint(level int, id string) (*entity.ChargePoint, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionChargePoints)
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
	cursor, err := collection.Aggregate(m.ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	if cursor.Next(m.ctx) {
		if err = cursor.Decode(&chargePoint); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("no charge point found")
	}
	return &chargePoint, nil
}

func (m *MongoDB) UpdateChargePoint(level int, chargePoint *entity.ChargePoint) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionChargePoints)
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
	result, err := collection.UpdateOne(m.ctx, filter, update)
	if err != nil {
		return fmt.Errorf("update charge point: %v", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("no charge point found")
	}

	// update connectors data
	collection = connection.Database(m.database).Collection(collectionConnectors)
	for _, connector := range chargePoint.Connectors {
		filter = bson.M{"charge_point_id": chargePoint.Id, "connector_id": connector.Id}
		update = bson.M{"$set": bson.D{
			{"type", connector.Type},
			{"power", connector.Power},
			{"connector_id_name", connector.IdName},
		}}
		_, err = collection.UpdateOne(m.ctx, filter, update)
	}
	return err
}

func (m *MongoDB) GetConnector(chargePointId string, connectorId int) (*entity.Connector, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionConnectors)
	filter := bson.D{{"charge_point_id", chargePointId}, {"connector_id", connectorId}}
	var connector entity.Connector
	if err = collection.FindOne(m.ctx, filter).Decode(&connector); err != nil {
		return nil, m.findError(err)
	}
	return &connector, nil
}

func (m *MongoDB) getTransactionState(userId string, level int, transaction *entity.Transaction) (*entity.ChargeState, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	var chargeState entity.ChargeState

	chargePoint, err := m.GetChargePoint(level, transaction.ChargePointId)
	if err != nil {
		return nil, fmt.Errorf("get charge point: %v", err)
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

		lastMeter, _ := m.GetLastMeterValue(transaction.TransactionId)
		if lastMeter != nil {
			if lastMeter.Value > transaction.MeterStart {
				consumed = lastMeter.Value - transaction.MeterStart
			}
			price = lastMeter.Price
			powerRate = lastMeter.PowerRate
		}

		meterValues, err = m.GetMeterValues(transaction.TransactionId, transaction.TimeStart)

		if len(meterValues) == 0 && lastMeter != nil {
			meterValues = append(meterValues, lastMeter)
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
		Status:             connector.Status,
		IsCharging:         transaction.IsFinished == false,
		CanStop:            canStop,
		MeterValues:        meterValues,
	}

	return &chargeState, nil
}

func (m *MongoDB) GetTransactionState(userId string, level int, id int) (*entity.ChargeState, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	transaction, err := m.GetTransaction(id)
	if err != nil {
		return nil, err
	}
	chargeState, err := m.getTransactionState(userId, level, transaction)
	if err != nil {
		return nil, err
	}
	return chargeState, nil
}

func (m *MongoDB) GetTransaction(id int) (*entity.Transaction, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionTransactions)
	filter := bson.D{{"transaction_id", id}}
	var transaction entity.Transaction
	if err = collection.FindOne(m.ctx, filter).Decode(&transaction); err != nil {
		return nil, m.findError(err)
	}
	return &transaction, nil
}

// UpdateTransaction update transaction billed data
func (m *MongoDB) UpdateTransaction(transaction *entity.Transaction) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionTransactions)
	filter := bson.D{{"transaction_id", transaction.TransactionId}}
	update := bson.D{
		{"$set", bson.D{
			{"payment_order", transaction.PaymentOrder},
			{"payment_billed", transaction.PaymentBilled},
		}},
	}
	if _, err = collection.UpdateOne(m.ctx, filter, update); err != nil {
		return err
	}
	return nil
}

func (m *MongoDB) GetActiveTransactions(userId string) ([]*entity.ChargeState, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	user, err := m.GetUserById(userId)
	if err != nil {
		return nil, fmt.Errorf("get user: %v", err)
	}

	tags, err := m.GetUserTags(userId)
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

	collection := connection.Database(m.database).Collection(collectionTransactions)
	filter := bson.D{
		{"id_tag", bson.D{{"$in", idTags}}},
		{"is_finished", false},
	}
	var transactions []entity.Transaction
	cursor, err := collection.Find(m.ctx, filter)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(m.ctx, &transactions); err != nil {
		return nil, err
	}
	if len(transactions) == 0 {
		return nil, nil
	}

	var chargeStates []*entity.ChargeState
	for _, transaction := range transactions {
		chargeState, _ := m.getTransactionState(userId, user.AccessLevel, &transaction)
		if chargeState != nil {
			chargeStates = append(chargeStates, chargeState)
		}
	}
	return chargeStates, nil
}

// GetTransactionsToBill get list of transactions to bill
func (m *MongoDB) GetTransactionsToBill(userId string) ([]*entity.Transaction, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	tags, err := m.GetUserTags(userId)
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

	collection := connection.Database(m.database).Collection(collectionTransactions)
	filter := bson.D{
		{"id_tag", bson.D{{"$in", idTags}}},
		{"is_finished", true},
		{"$expr", bson.D{{"$lt", bson.A{"$payment_billed", "$payment_amount"}}}},
	}
	var transactions []*entity.Transaction
	cursor, err := collection.Find(m.ctx, filter)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(m.ctx, &transactions); err != nil {
		return nil, err
	}
	return transactions, nil
}

// GetTransactions gets list of a user's transactions; if period is empty, returns last 100 transactions
func (m *MongoDB) GetTransactions(userId string, period string) ([]*entity.Transaction, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	tags, err := m.GetUserTags(userId)
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

	collection := connection.Database(m.database).Collection(collectionTransactions)

	var transactions []*entity.Transaction
	var cursor *mongo.Cursor

	time1, time2, err := parsePeriod(period)
	if err != nil {
		filter := bson.D{
			{"id_tag", bson.D{{"$in", idTags}}},
			{"is_finished", true},
		}
		cursor, err = collection.Find(m.ctx, filter, options.Find().SetLimit(int64(100)).SetSkip(int64(0)).SetSort(bson.D{{"time_start", -1}}))
	} else {
		filter := bson.D{
			{"id_tag", bson.D{{"$in", idTags}}},
			{"is_finished", true},
			{"time_start", bson.D{{"$gte", time1}}},
			{"time_start", bson.D{{"$lte", time2}}},
		}
		cursor, err = collection.Find(m.ctx, filter, options.Find().SetSort(bson.D{{"time_start", -1}}))
	}

	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(m.ctx, &transactions); err != nil {
		return nil, err
	}
	return transactions, nil
}

func (m *MongoDB) GetTransactionByTag(idTag string, timeStart time.Time) (*entity.Transaction, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionTransactions)
	filter := bson.D{
		{"id_tag", strings.ToUpper(idTag)},
		{"time_start", bson.D{{"$gte", timeStart}}},
	}
	var transaction entity.Transaction
	if err = collection.FindOne(m.ctx, filter).Decode(&transaction); err != nil {
		return nil, m.findError(err)
	}
	return &transaction, nil
}

func (m *MongoDB) GetLastMeterValue(transactionId int) (*entity.TransactionMeter, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionMeterValues)
	filter := bson.D{
		{"transaction_id", transactionId},
		{"measurand", "Energy.Active.Import.Register"},
	}
	var value entity.TransactionMeter
	if err = collection.FindOne(m.ctx, filter, options.FindOne().SetSort(bson.D{{"time", -1}})).Decode(&value); err != nil {
		return nil, m.findError(err)
	}
	return &value, nil
}

func (m *MongoDB) GetMeterValues(transactionId int, from time.Time) ([]*entity.TransactionMeter, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	filter := bson.D{
		{"transaction_id", transactionId},
		{"measurand", "Energy.Active.Import.Register"},
		{"time", bson.D{{"$gte", from}}},
	}
	collection := connection.Database(m.database).Collection(collectionMeterValues)
	opts := options.Find().SetSort(bson.D{{"time", 1}})
	cursor, err := collection.Find(m.ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var meterValues []*entity.TransactionMeter
	if err = cursor.All(m.ctx, &meterValues); err != nil {
		return nil, m.findError(err)
	}
	return meterValues, nil
}

func (m *MongoDB) AddInviteCode(invite *entity.Invite) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionInvites)
	_, err = collection.InsertOne(m.ctx, invite)
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoDB) CheckInviteCode(code string) (bool, error) {
	connection, err := m.connect()
	if err != nil {
		return false, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionInvites)
	filter := bson.D{{"code", code}}
	var inviteCode entity.Invite
	if err = collection.FindOne(m.ctx, filter).Decode(&inviteCode); err != nil {
		return false, err
	}
	return true, nil
}

func (m *MongoDB) DeleteInviteCode(code string) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionInvites)
	filter := bson.D{{"code", code}}
	_, err = collection.DeleteOne(m.ctx, filter)
	return err
}

func (m *MongoDB) SavePaymentResult(paymentParameters *entity.PaymentParameters) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionPayment)
	_, err = collection.InsertOne(m.ctx, paymentParameters)
	return err
}

// GetPaymentParameters get payment parameters by order id
func (m *MongoDB) GetPaymentParameters(orderId string) (*entity.PaymentParameters, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionPayment)
	filter := bson.D{{"order", orderId}}
	var paymentParameters entity.PaymentParameters
	if err = collection.FindOne(m.ctx, filter).Decode(&paymentParameters); err != nil {
		return nil, m.findError(err)
	}
	return &paymentParameters, nil
}

func (m *MongoDB) SavePaymentMethod(paymentMethod *entity.PaymentMethod) error {
	saved, _ := m.GetPaymentMethod(paymentMethod.Identifier, paymentMethod.UserId)
	if saved != nil {
		return fmt.Errorf("payment method with identifier %s... already exists", paymentMethod.Identifier[0:10])
	}

	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionPaymentMethods)
	_, err = collection.InsertOne(m.ctx, paymentMethod)
	if err != nil {
		return err
	}
	return nil
}

// UpdatePaymentMethod update user-editable fields for payment method
func (m *MongoDB) UpdatePaymentMethod(paymentMethod *entity.PaymentMethod) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionPaymentMethods)

	// if current method is default, then set all other methods to not default
	if paymentMethod.IsDefault {
		filter := bson.D{{"user_id", paymentMethod.UserId}}
		update := bson.M{"$set": bson.D{{"is_default", false}}}
		_, err = collection.UpdateMany(m.ctx, filter, update)
		if err != nil {
			return err
		}
	}

	filter := bson.D{{"identifier", paymentMethod.Identifier}, {"user_id", paymentMethod.UserId}}
	update := bson.M{"$set": bson.D{
		{"description", paymentMethod.Description},
		{"is_default", paymentMethod.IsDefault},
	}}
	_, err = collection.UpdateOne(m.ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

// DeletePaymentMethod delete payment method by identifier
func (m *MongoDB) DeletePaymentMethod(paymentMethod *entity.PaymentMethod) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	isDefault := paymentMethod.IsDefault

	collection := connection.Database(m.database).Collection(collectionPaymentMethods)
	filter := bson.D{{"identifier", paymentMethod.Identifier}, {"user_id", paymentMethod.UserId}}
	result, err := collection.DeleteOne(m.ctx, filter)
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
		err = collection.FindOne(m.ctx, filter).Decode(&method)
		if err == nil {
			filter = bson.D{{"identifier", method.Identifier}, {"user_id", method.UserId}}
			update := bson.M{"$set": bson.D{
				{"is_default", true},
			}}
			_, _ = collection.UpdateOne(m.ctx, filter, update)
		}
	}
	return nil
}

// GetPaymentMethods get payment methods by user id
func (m *MongoDB) GetPaymentMethods(userId string) ([]*entity.PaymentMethod, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionPaymentMethods)
	filter := bson.D{{"user_id", userId}}
	var paymentMethods []*entity.PaymentMethod
	cursor, err := collection.Find(m.ctx, filter)
	if err != nil {
		return nil, m.findError(err)
	}
	if err = cursor.All(m.ctx, &paymentMethods); err != nil {
		return nil, err
	}
	return paymentMethods, nil
}

// GetPaymentMethod get payment method by identifier
func (m *MongoDB) GetPaymentMethod(identifier, userId string) (*entity.PaymentMethod, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionPaymentMethods)
	filter := bson.D{{"identifier", identifier}, {"user_id", userId}}
	var paymentMethod entity.PaymentMethod
	if err = collection.FindOne(m.ctx, filter).Decode(&paymentMethod); err != nil {
		return nil, m.findError(err)
	}
	return &paymentMethod, nil
}

func (m *MongoDB) GetLastOrder() (*entity.PaymentOrder, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionPaymentOrders)
	filter := bson.D{}
	var order entity.PaymentOrder
	if err = collection.FindOne(m.ctx, filter, options.FindOne().SetSort(bson.D{{"time_opened", -1}})).Decode(&order); err != nil {
		return nil, m.findError(err)
	}
	return &order, nil
}

// GetPaymentOrderByTransaction get payment order by transaction id
func (m *MongoDB) GetPaymentOrderByTransaction(transactionId int) (*entity.PaymentOrder, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionPaymentOrders)
	filter := bson.D{{"transaction_id", transactionId}, {"is_completed", false}}
	var order entity.PaymentOrder
	if err = collection.FindOne(m.ctx, filter).Decode(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (m *MongoDB) GetPaymentOrder(id int) (*entity.PaymentOrder, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionPaymentOrders)
	filter := bson.D{{"order", id}}
	var order entity.PaymentOrder
	if err = collection.FindOne(m.ctx, filter).Decode(&order); err != nil {
		return nil, m.findError(err)
	}
	return &order, nil
}

// SavePaymentOrder insert or update order
func (m *MongoDB) SavePaymentOrder(order *entity.PaymentOrder) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	filter := bson.D{{"order", order.Order}}
	set := bson.M{"$set": order}
	collection := connection.Database(m.database).Collection(collectionPaymentOrders)
	_, err = collection.UpdateOne(m.ctx, filter, set, options.Update().SetUpsert(true))
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
func (m *MongoDB) GetLocations() ([]*entity.Location, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

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
	collection := connection.Database(m.database).Collection(collectionLocations)
	cursor, err := collection.Aggregate(m.ctx, pipeline)
	if err != nil {
		return nil, m.findError(err)
	}
	var locations []*entity.Location
	if err = cursor.All(m.ctx, &locations); err != nil {
		return nil, err
	}
	return locations, nil
}
