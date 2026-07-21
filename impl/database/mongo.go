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
	collectionSysLog            = "sys_log"
	collectionBackLog           = "back_log"
	collectionPaymentLog        = "payment_log"
	collectionConfig            = "config"
	collectionUsers             = "users"
	collectionUserTags          = "user_tags"
	collectionChargePoints      = "charge_points"
	collectionConnectors        = "connectors"
	collectionTransactions      = "transactions"
	collectionMeterValues       = "meter_values"
	collectionInvites           = "invites"
	collectionPayment           = "payment"
	collectionPaymentMethods    = "payment_methods"
	collectionPaymentOrders     = "payment_orders"
	collectionPaymentPlans      = "payment_plans"
	collectionLocations         = "locations"
	collectionPreauthorizations = "preauthorizations"
	collectionPaymentRetries    = "payment_retries"
	collectionMailSubscriptions = "mail_subscriptions"
)

type MongoDB struct {
	client           *mongo.Client
	database         string
	logRecordsNumber int64
}

func (m *MongoDB) col(name string) *mongo.Collection {
	return m.client.Database(m.database).Collection(name)
}

func findOne[T any](m *MongoDB, ctx context.Context, colName string, filter any) (*T, error) {
	var data T
	if err := m.col(colName).FindOne(ctx, filter).Decode(&data); err != nil {
		return nil, m.findError(err)
	}
	return &data, nil
}

// findMany runs a Find and decodes all matching documents into a []T.
func findMany[T any](m *MongoDB, ctx context.Context, colName string, filter any, opts ...*options.FindOptions) ([]T, error) {
	cursor, err := m.col(colName).Find(ctx, filter, opts...)
	if err != nil {
		return nil, m.findError(err)
	}
	var out []T
	if err = cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// aggregateMany runs an aggregation pipeline and decodes all documents into a []T.
func aggregateMany[T any](m *MongoDB, ctx context.Context, colName string, pipeline any, opts ...*options.AggregateOptions) ([]T, error) {
	cursor, err := m.col(colName).Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return nil, m.findError(err)
	}
	var out []T
	if err = cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// updateOne updates a single document; resourceName is the noun used to build
// a "<resource> not found" error wrapping entity.ErrNotFound when no document matches.
func (m *MongoDB) updateOne(ctx context.Context, colName string, filter, update any, resourceName string) error {
	result, err := m.col(colName).UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("%s %w", resourceName, entity.ErrNotFound)
	}
	return nil
}

func (m *MongoDB) deleteOne(ctx context.Context, colName string, filter any, resourceName string) error {
	result, err := m.col(colName).DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("%s %w", resourceName, entity.ErrNotFound)
	}
	return nil
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

func (m *MongoDB) read(ctx context.Context, table, dataType string) (any, error) {
	var logMessages any
	timeFieldName := "timestamp"

	switch dataType {
	case entity.FeatureMessageType:
		logMessages = []entity.FeatureMessage{}
	case entity.LogMessageType:
		logMessages = []entity.LogMessage{}
	default:
		return nil, fmt.Errorf("unknown data type: %s", dataType)
	}

	collection := m.col(table)
	filter := bson.D{}
	opts := options.Find().SetSort(bson.D{{Key: timeFieldName, Value: -1}})
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

func (m *MongoDB) ReadLog(ctx context.Context, logName string) (any, error) {
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
	var pipe []bson.M
	pipe = append(pipe, bson.M{"$match": bson.M{"timestamp": bson.M{"$gt": timeStart}}})
	pipe = append(pipe, bson.M{"$sort": bson.M{"timestamp": 1}})
	pipe = append(pipe, bson.M{"$limit": m.logRecordsNumber})
	result, err := aggregateMany[*entity.FeatureMessage](m, ctx, collectionSysLog, pipe)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *MongoDB) GetConfig(ctx context.Context, name string) (any, error) {
	collection := m.col(collectionConfig)
	filter := bson.D{{Key: "name", Value: name}}
	var configData any
	err := collection.FindOne(ctx, filter).Decode(&configData)
	if err != nil {
		return nil, m.findError(err)
	}
	return configData, nil
}

func (m *MongoDB) GetUser(ctx context.Context, username string) (*entity.User, error) {
	return findOne[entity.User](m, ctx, collectionUsers, bson.D{{Key: "username", Value: username}})
}

// GetUserInfo get user info: data of a user, merged with tags, payment methods, payment plans
func (m *MongoDB) GetUserInfo(ctx context.Context, _ int, username string) (*entity.UserInfo, error) {
	pipeline := mongo.Pipeline{
		{
			{Key: "$match", Value: bson.M{"username": username}},
			//	"$and": []bson.M{
			//		{"username": username},
			//		{"access_level": bson.M{"$lte": level}},
			//	},
			//}},
		},
		{{Key: "$lookup", Value: bson.M{
			"from":         collectionPaymentPlans,
			"localField":   "payment_plan",
			"foreignField": "plan_id",
			"as":           "payment_plans",
		}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         collectionPaymentMethods,
			"localField":   "username",
			"foreignField": "user_name",
			"as":           "payment_methods",
		}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         collectionUserTags,
			"localField":   "username",
			"foreignField": "username",
			"as":           "user_tags",
		}}},
		{{Key: "$project", Value: bson.M{
			//"payment_methods.identifier": 0,
			"payment_methods.user_id": 0,
			"user_tags.user_id":       0,
		}}},
	}
	collection := m.col(collectionUsers)
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
	projection := bson.M{"password": 0, "token": 0}
	return findMany[*entity.User](m, ctx, collectionUsers, bson.D{}, options.Find().SetProjection(projection))
}

func (m *MongoDB) GetUserById(ctx context.Context, userId string) (*entity.User, error) {
	return findOne[entity.User](m, ctx, collectionUsers, bson.D{{Key: "user_id", Value: userId}})
}

// GetWarningEmailRecipients returns users who opted in to payment warning emails.
func (m *MongoDB) GetWarningEmailRecipients(ctx context.Context) ([]*entity.User, error) {
	filter := bson.D{{Key: "warning_emails_enabled", Value: true}}
	projection := bson.M{"password": 0, "token": 0}
	return findMany[*entity.User](m, ctx, collectionUsers, filter, options.Find().SetProjection(projection))
}

func (m *MongoDB) GetUserTags(ctx context.Context, userId string) ([]entity.UserTag, error) {
	filter := bson.M{"$and": []bson.M{{"user_id": userId}, {"is_enabled": true}}}
	return findMany[entity.UserTag](m, ctx, collectionUserTags, filter)
}

func (m *MongoDB) GetUserTag(ctx context.Context, idTag string) (*entity.UserTag, error) {
	return findOne[entity.UserTag](m, ctx, collectionUserTags, bson.D{{Key: "id_tag", Value: idTag}})
}

func (m *MongoDB) AddUserTag(ctx context.Context, userTag *entity.UserTag) error {
	t, _ := m.GetUserTag(ctx, userTag.IdTag)
	if t != nil {
		return fmt.Errorf("user tag %s already exists for user %s", t.IdTag, t.UserId)
	}
	collection := m.col(collectionUserTags)
	_, err := collection.InsertOne(ctx, userTag)
	return err
}

// CheckUserTag check unique of idTag
func (m *MongoDB) CheckUserTag(ctx context.Context, idTag string) error {
	collection := m.col(collectionUserTags)
	filter := bson.D{{Key: "id_tag", Value: idTag}}
	var userTag entity.UserTag
	err := collection.FindOne(ctx, filter).Decode(&userTag)
	if err == nil {
		return fmt.Errorf("user tag %s already exists", userTag.IdTag)
	}
	return nil
}

func (m *MongoDB) UpdateTagLastSeen(ctx context.Context, userTag *entity.UserTag) error {
	collection := m.col(collectionUserTags)
	filter := bson.D{{Key: "id_tag", Value: userTag.IdTag}}
	update := bson.M{"$set": bson.D{
		{Key: "last_seen", Value: time.Now()},
	}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (m *MongoDB) GetAllUserTags(ctx context.Context) ([]*entity.UserTag, error) {
	return findMany[*entity.UserTag](m, ctx, collectionUserTags, bson.M{})
}

func (m *MongoDB) GetUserTagByIdTag(ctx context.Context, idTag string) (*entity.UserTag, error) {
	return m.GetUserTag(ctx, idTag)
}

func (m *MongoDB) UpdateUserTag(ctx context.Context, userTag *entity.UserTag) error {
	filter := bson.D{{Key: "id_tag", Value: userTag.IdTag}}
	update := bson.M{"$set": bson.D{
		{Key: "username", Value: userTag.Username},
		{Key: "user_id", Value: userTag.UserId},
		{Key: "source", Value: userTag.Source},
		{Key: "is_enabled", Value: userTag.IsEnabled},
		{Key: "local", Value: userTag.Local},
		{Key: "note", Value: userTag.Note},
	}}
	return m.updateOne(ctx, collectionUserTags, filter, update, "tag")
}

func (m *MongoDB) DeleteUserTag(ctx context.Context, idTag string) error {
	return m.deleteOne(ctx, collectionUserTags, bson.D{{Key: "id_tag", Value: idTag}}, "tag")
}

func (m *MongoDB) UpdateLastSeen(ctx context.Context, user *entity.User) error {
	collection := m.col(collectionUsers)
	filter := bson.D{{Key: "username", Value: user.Username}}
	update := bson.M{"$set": bson.D{
		{Key: "last_seen", Value: time.Now()},
		{Key: "token", Value: user.Token},
	}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (m *MongoDB) AddUser(ctx context.Context, user *entity.User) error {
	collection := m.col(collectionUsers)
	_, err := collection.InsertOne(ctx, user)
	return err
}

func (m *MongoDB) UpdateUser(ctx context.Context, user *entity.User) error {
	filter := bson.D{{Key: "username", Value: user.Username}}
	update := bson.M{"$set": bson.D{
		{Key: "name", Value: user.Name},
		{Key: "email", Value: user.Email},
		{Key: "role", Value: user.Role},
		{Key: "access_level", Value: user.AccessLevel},
		{Key: "payment_plan", Value: user.PaymentPlan},
		{Key: "password", Value: user.Password},
		{Key: "warning_emails_enabled", Value: user.WarningEmailsEnabled},
		{Key: "warning_email", Value: user.WarningEmail},
	}}
	return m.updateOne(ctx, collectionUsers, filter, update, "user")
}

func (m *MongoDB) DeleteUser(ctx context.Context, username string) error {
	return m.deleteOne(ctx, collectionUsers, bson.D{{Key: "username", Value: username}}, "user")
}

// CheckUsername check unique username
func (m *MongoDB) CheckUsername(ctx context.Context, username string) error {
	collection := m.col(collectionUsers)
	filter := bson.D{{Key: "username", Value: username}}
	var userData entity.User
	err := collection.FindOne(ctx, filter).Decode(&userData)
	if err == nil {
		return fmt.Errorf("username %s already exists", username)
	}
	return nil
}

func (m *MongoDB) CheckToken(ctx context.Context, token string) (*entity.User, error) {
	collection := m.col(collectionUsers)
	filter := bson.D{{Key: "token", Value: token}}
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
			{Key: "$match", Value: bson.M{
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
		{{Key: "$lookup", Value: bson.M{
			"from":         collectionConnectors,
			"localField":   "charge_point_id",
			"foreignField": "charge_point_id",
			"as":           "Connectors",
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "charge_point_id", Value: 1}}}},
	}

	return aggregateMany[*entity.ChargePoint](m, ctx, collectionChargePoints, pipeline)
}

func (m *MongoDB) GetChargePoint(ctx context.Context, level int, id string) (*entity.ChargePoint, error) {
	collection := m.col(collectionChargePoints)
	filter := bson.M{"$and": []bson.M{{"charge_point_id": id}, {"access_level": bson.M{"$lte": level}}}}

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: filter}}, // Match the charge point
		bson.D{{Key: "$lookup", Value: bson.D{ // Lookup to join with connectors collection
			{Key: "from", Value: collectionConnectors},      // The collection to join
			{Key: "localField", Value: "charge_point_id"},   // Field from charge_points collection
			{Key: "foreignField", Value: "charge_point_id"}, // Field from connectors collection
			{Key: "as", Value: "Connectors"},                // The array field to add in chargePoint
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
	collection := m.col(collectionChargePoints)
	filter := bson.M{"$and": []bson.M{
		{"charge_point_id": chargePoint.Id},
		{"access_level": bson.M{"$lte": level}},
	}}
	fields := bson.D{
		{Key: "title", Value: chargePoint.Title},
		{Key: "description", Value: chargePoint.Description},
		{Key: "address", Value: chargePoint.Address},
		{Key: "access_type", Value: chargePoint.AccessType},
		{Key: "access_level", Value: chargePoint.AccessLevel},
		{Key: "location.latitude", Value: chargePoint.Location.Latitude},
		{Key: "location.longitude", Value: chargePoint.Location.Longitude},
	}
	// Only written when the request carried it, so clients that don't know the
	// field cannot silently disable meter value triggering.
	if chargePoint.TriggerMessage != nil {
		fields = append(fields, bson.E{Key: "trigger_message", Value: *chargePoint.TriggerMessage})
	}
	update := bson.M{"$set": fields}
	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("update charge point: %v", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("no charge point found")
	}

	// update connectors data
	collection = m.col(collectionConnectors)
	for _, connector := range chargePoint.Connectors {
		filter = bson.M{"charge_point_id": chargePoint.Id, "connector_id": connector.Id}
		update = bson.M{"$set": bson.D{
			{Key: "type", Value: connector.Type},
			{Key: "power", Value: connector.Power},
			{Key: "connector_id_name", Value: connector.IdName},
		}}
		_, err = collection.UpdateOne(ctx, filter, update)
	}
	return err
}

func (m *MongoDB) GetConnector(chargePointId string, connectorId int) (*entity.Connector, error) {
	return findOne[entity.Connector](m, context.Background(), collectionConnectors, bson.D{{Key: "charge_point_id", Value: chargePointId}, {Key: "connector_id", Value: connectorId}})
}

func (m *MongoDB) getTransactionState(ctx context.Context, userId string, level int, transaction *entity.Transaction) (*entity.ChargeState, error) {
	var chargeState entity.ChargeState

	chargePoint, err := m.GetChargePoint(ctx, level, transaction.ChargePointId)
	if err != nil {
		return nil, fmt.Errorf("get charge point: %v", err)
	}
	if chargePoint == nil {
		return nil, fmt.Errorf("charge point %w", entity.ErrNotFound)
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
		PowerLimit:         transaction.PowerLimit,
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

		SessionId:       transaction.SessionId,
		IsFinished:      transaction.IsFinished,
		Username:        transaction.Username,
		IdTagNote:       transaction.IdTagNote,
		UserTag:         transaction.UserTag,
		ReservationId:   transaction.ReservationId,
		MeterStop:       transaction.MeterStop,
		TimeStop:        transaction.TimeStop,
		Reason:          transaction.Reason,
		PaymentAmount:   transaction.PaymentAmount,
		PaymentOrder:    transaction.PaymentOrder,
		PaymentError:    transaction.PaymentError,
		Tariff:          transaction.Tariff,
		ProtocolVersion: transaction.ProtocolVersion,
		EvseId:          transaction.EvseId,
		Metadata:        transaction.Metadata,
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
	return findOne[entity.Transaction](m, ctx, collectionTransactions, bson.D{{Key: "transaction_id", Value: id}})
}

// UpdateTransactionPayment updates payment-related fields on a transaction.
func (m *MongoDB) UpdateTransactionPayment(ctx context.Context, transaction *entity.Transaction) error {
	collection := m.col(collectionTransactions)
	filter := bson.D{{Key: "transaction_id", Value: transaction.TransactionId}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "payment_order", Value: transaction.PaymentOrder},
			{Key: "payment_billed", Value: transaction.PaymentBilled},
			{Key: "payment_error", Value: transaction.PaymentError},
			{Key: "payment_orders", Value: transaction.PaymentOrders},
			{Key: "payment_method", Value: transaction.PaymentMethod},
		}},
	}
	if _, err := collection.UpdateOne(ctx, filter, update); err != nil {
		return err
	}
	return nil
}

// userIdTags returns the user's uppercased id_tags. Returns (nil, nil) when the user has no tags.
func (m *MongoDB) userIdTags(ctx context.Context, userId string) ([]string, error) {
	tags, err := m.GetUserTags(ctx, userId)
	if err != nil {
		return nil, err
	}
	idTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		idTags = append(idTags, strings.ToUpper(tag.IdTag))
	}
	return idTags, nil
}

func (m *MongoDB) GetActiveTransactions(ctx context.Context, userId string) ([]*entity.ChargeState, error) {
	user, err := m.GetUserById(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("get user: %v", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user %w", entity.ErrNotFound)
	}

	idTags, err := m.userIdTags(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("get user tags: %v", err)
	}
	if len(idTags) == 0 {
		return nil, nil
	}

	filter := bson.D{
		{Key: "id_tag", Value: bson.D{{Key: "$in", Value: idTags}}},
		{Key: "is_finished", Value: false},
	}
	transactions, err := findMany[entity.Transaction](m, ctx, collectionTransactions, filter)
	if err != nil {
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
	idTags, err := m.userIdTags(ctx, userId)
	if err != nil {
		return nil, err
	}
	if len(idTags) == 0 {
		return nil, nil
	}

	filter := bson.D{
		{Key: "id_tag", Value: bson.D{{Key: "$in", Value: idTags}}},
		{Key: "is_finished", Value: true},
		{Key: "$expr", Value: bson.D{{Key: "$lt", Value: bson.A{"$payment_billed", "$payment_amount"}}}},
	}
	return findMany[*entity.Transaction](m, ctx, collectionTransactions, filter)
}

// GetTransactions gets list of a user's transactions; if period is empty, returns last 100 transactions
func (m *MongoDB) GetTransactions(ctx context.Context, userId string, period string) ([]*entity.Transaction, error) {
	idTags, err := m.userIdTags(ctx, userId)
	if err != nil {
		return nil, err
	}
	if len(idTags) == 0 {
		return nil, nil
	}

	var filter bson.D
	var opts *options.FindOptions

	time1, time2, err := parsePeriod(period)
	if err != nil {
		filter = bson.D{
			{Key: "id_tag", Value: bson.D{{Key: "$in", Value: idTags}}},
			{Key: "is_finished", Value: true},
		}
		opts = options.Find().SetLimit(int64(100)).SetSkip(int64(0)).SetSort(bson.D{{Key: "time_start", Value: -1}})
	} else {
		filter = bson.D{
			{Key: "id_tag", Value: bson.D{{Key: "$in", Value: idTags}}},
			{Key: "is_finished", Value: true},
			{Key: "time_stop", Value: bson.D{{Key: "$gte", Value: time1}}},
			{Key: "time_stop", Value: bson.D{{Key: "$lte", Value: time2}}},
		}
		opts = options.Find().SetSort(bson.D{{Key: "time_start", Value: -1}})
	}

	return findMany[*entity.Transaction](m, ctx, collectionTransactions, filter, opts)
}

// GetFilteredTransactions returns transactions filtered by the provided criteria
func (m *MongoDB) GetFilteredTransactions(ctx context.Context, filter *entity.TransactionFilter) ([]*entity.Transaction, error) {
	// Build dynamic filter
	mongoFilter := bson.D{}

	// Filter by username: lookup user's id_tags first
	if filter.Username != "" {
		tagFilter := bson.D{{Key: "username", Value: filter.Username}}
		userTags, err := findMany[*entity.UserTag](m, ctx, collectionUserTags, tagFilter)
		if err != nil {
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
		mongoFilter = append(mongoFilter, bson.E{Key: "id_tag", Value: bson.D{{Key: "$in", Value: idTags}}})
	}

	// Filter by id_tag
	if filter.IdTag != "" {
		mongoFilter = append(mongoFilter, bson.E{Key: "id_tag", Value: strings.ToUpper(filter.IdTag)})
	}

	// Filter by charge_point_id
	if filter.ChargePointId != "" {
		mongoFilter = append(mongoFilter, bson.E{Key: "charge_point_id", Value: filter.ChargePointId})
	}

	// Filter by date range
	if filter.From != nil {
		mongoFilter = append(mongoFilter, bson.E{Key: "time_stop", Value: bson.D{{Key: "$gte", Value: *filter.From}}})
	}
	if filter.To != nil {
		mongoFilter = append(mongoFilter, bson.E{Key: "time_stop", Value: bson.D{{Key: "$lte", Value: *filter.To}}})
	}

	// Filter by payment_error (non-empty)
	if filter.WithError {
		mongoFilter = append(mongoFilter, bson.E{Key: "payment_error", Value: bson.D{{Key: "$exists", Value: true}, {Key: "$ne", Value: ""}}})
	}

	// Only finished transactions
	mongoFilter = append(mongoFilter, bson.E{Key: "is_finished", Value: true})

	// Sort by time_start descending (newest first)
	opts := options.Find().SetSort(bson.D{{Key: "time_start", Value: -1}})

	return findMany[*entity.Transaction](m, ctx, collectionTransactions, mongoFilter, opts)
}

func (m *MongoDB) GetTransactionByTag(ctx context.Context, idTag string, timeStart time.Time) (*entity.Transaction, error) {
	collection := m.col(collectionTransactions)
	filter := bson.D{
		{Key: "id_tag", Value: strings.ToUpper(idTag)},
		{Key: "time_start", Value: bson.D{{Key: "$gte", Value: timeStart}}},
	}
	var transaction entity.Transaction
	if err := collection.FindOne(ctx, filter).Decode(&transaction); err != nil {
		return nil, m.findError(err)
	}
	return &transaction, nil
}

func (m *MongoDB) GetLastMeterValue(ctx context.Context, transactionId int) (*entity.TransactionMeter, error) {
	collection := m.col(collectionMeterValues)
	filter := bson.D{
		{Key: "transaction_id", Value: transactionId},
		{Key: "measurand", Value: "Energy.Active.Import.Register"},
	}
	var value entity.TransactionMeter
	if err := collection.FindOne(ctx, filter, options.FindOne().SetSort(bson.D{{Key: "time", Value: -1}})).Decode(&value); err != nil {
		return nil, m.findError(err)
	}
	return &value, nil
}

func (m *MongoDB) GetMeterValues(ctx context.Context, transactionId int, from time.Time) ([]entity.TransactionMeter, error) {
	filter := bson.D{
		{Key: "transaction_id", Value: transactionId},
		{Key: "measurand", Value: "Energy.Active.Import.Register"},
		{Key: "time", Value: bson.D{{Key: "$gte", Value: from}}},
	}
	opts := options.Find().SetSort(bson.D{{Key: "time", Value: 1}})
	return findMany[entity.TransactionMeter](m, ctx, collectionMeterValues, filter, opts)
}

func (m *MongoDB) GetRecentUserChargePoints(ctx context.Context, userId string) ([]*entity.ChargePoint, error) {
	// Get last 3 transactions for user's tags
	idTags, err := m.userIdTags(ctx, userId)
	if err != nil {
		return nil, err
	}
	if len(idTags) == 0 {
		return nil, nil
	}

	// Get transactions
	filter := bson.D{
		{Key: "id_tag", Value: bson.D{{Key: "$in", Value: idTags}}},
		{Key: "is_finished", Value: true},
	}
	opts := options.Find().SetSort(bson.D{{Key: "time_start", Value: -1}}).SetLimit(3)
	transactions, err := findMany[entity.Transaction](m, ctx, collectionTransactions, filter, opts)
	if err != nil {
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
		{{Key: "$match", Value: bson.D{{Key: "charge_point_id", Value: bson.D{{Key: "$in", Value: chargePointIds}}}}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         collectionConnectors,
			"localField":   "charge_point_id",
			"foreignField": "charge_point_id",
			"as":           "Connectors",
		}}},
	}

	return aggregateMany[*entity.ChargePoint](m, ctx, collectionChargePoints, pipeline)
}

func (m *MongoDB) AddInviteCode(ctx context.Context, invite *entity.Invite) error {
	collection := m.col(collectionInvites)
	_, err := collection.InsertOne(ctx, invite)
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoDB) CheckInviteCode(ctx context.Context, code string) (bool, error) {
	collection := m.col(collectionInvites)
	filter := bson.D{{Key: "code", Value: code}}
	var inviteCode entity.Invite
	if err := collection.FindOne(ctx, filter).Decode(&inviteCode); err != nil {
		return false, err
	}
	return true, nil
}

func (m *MongoDB) DeleteInviteCode(ctx context.Context, code string) error {
	collection := m.col(collectionInvites)
	filter := bson.D{{Key: "code", Value: code}}
	_, err := collection.DeleteOne(ctx, filter)
	return err
}

func (m *MongoDB) SavePaymentResult(ctx context.Context, paymentParameters *entity.PaymentParameters) error {
	collection := m.col(collectionPayment)
	_, err := collection.InsertOne(ctx, paymentParameters)
	return err
}

// WritePaymentLog inserts a single payment activity log entry.
func (m *MongoDB) WritePaymentLog(ctx context.Context, msg *entity.LogMessage) error {
	collection := m.col(collectionPaymentLog)
	_, err := collection.InsertOne(ctx, msg)
	return err
}

// GetPaymentParameters get payment parameters by order id
func (m *MongoDB) GetPaymentParameters(orderId string) (*entity.PaymentParameters, error) {
	return findOne[entity.PaymentParameters](m, context.Background(), collectionPayment, bson.D{{Key: "order", Value: orderId}})
}

func (m *MongoDB) SavePaymentMethod(ctx context.Context, paymentMethod *entity.PaymentMethod) error {
	saved, _ := m.GetPaymentMethod(ctx, paymentMethod.Identifier, paymentMethod.UserId)
	if saved != nil {
		return fmt.Errorf("payment method with identifier %s... already exists", paymentMethod.Identifier[0:10])
	}

	collection := m.col(collectionPaymentMethods)
	_, err := collection.InsertOne(ctx, paymentMethod)
	if err != nil {
		return err
	}
	return nil
}

// UpdatePaymentMethod update user-editable fields for payment method
func (m *MongoDB) UpdatePaymentMethod(ctx context.Context, paymentMethod *entity.PaymentMethod) error {
	collection := m.col(collectionPaymentMethods)

	// if current method is default, then set all other methods to not default
	if paymentMethod.IsDefault {
		filter := bson.D{{Key: "user_id", Value: paymentMethod.UserId}}
		update := bson.M{"$set": bson.D{{Key: "is_default", Value: false}}}
		_, err := collection.UpdateMany(ctx, filter, update)
		if err != nil {
			return err
		}
	}

	filter := bson.D{{Key: "identifier", Value: paymentMethod.Identifier}, {Key: "user_id", Value: paymentMethod.UserId}}
	update := bson.M{"$set": bson.D{
		{Key: "description", Value: paymentMethod.Description},
		{Key: "is_default", Value: paymentMethod.IsDefault},
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

	collection := m.col(collectionPaymentMethods)
	filter := bson.D{{Key: "identifier", Value: paymentMethod.Identifier}, {Key: "user_id", Value: paymentMethod.UserId}}
	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("payment method %w", entity.ErrNotFound)
	}

	// if we deleted default method, then set first method to default
	if isDefault {
		var method entity.PaymentMethod
		filter = bson.D{{Key: "user_id", Value: paymentMethod.UserId}}
		err = collection.FindOne(ctx, filter).Decode(&method)
		if err == nil {
			filter = bson.D{{Key: "identifier", Value: method.Identifier}, {Key: "user_id", Value: method.UserId}}
			update := bson.M{"$set": bson.D{
				{Key: "is_default", Value: true},
			}}
			_, _ = collection.UpdateOne(ctx, filter, update)
		}
	}
	return nil
}

// GetPaymentMethods get payment methods by user id
func (m *MongoDB) GetPaymentMethods(ctx context.Context, userId string) ([]*entity.PaymentMethod, error) {
	filter := bson.D{{Key: "user_id", Value: userId}}
	paymentMethods, err := findMany[*entity.PaymentMethod](m, ctx, collectionPaymentMethods, filter)
	if err != nil {
		return nil, err
	}
	if paymentMethods == nil {
		paymentMethods = make([]*entity.PaymentMethod, 0)
	}
	return paymentMethods, nil
}

// GetPaymentMethod get payment method by identifier
func (m *MongoDB) GetPaymentMethod(ctx context.Context, identifier, userId string) (*entity.PaymentMethod, error) {
	return findOne[entity.PaymentMethod](m, ctx, collectionPaymentMethods, bson.D{{Key: "identifier", Value: identifier}, {Key: "user_id", Value: userId}})
}

func (m *MongoDB) GetLastOrder(ctx context.Context) (*entity.PaymentOrder, error) {
	collection := m.col(collectionPaymentOrders)
	filter := bson.D{}
	var order entity.PaymentOrder
	if err := collection.FindOne(ctx, filter, options.FindOne().SetSort(bson.D{{Key: "time_opened", Value: -1}})).Decode(&order); err != nil {
		return nil, m.findError(err)
	}
	return &order, nil
}

// GetPaymentOrderByTransaction get payment order by transaction id
func (m *MongoDB) GetPaymentOrderByTransaction(ctx context.Context, transactionId int) (*entity.PaymentOrder, error) {
	collection := m.col(collectionPaymentOrders)
	filter := bson.D{{Key: "transaction_id", Value: transactionId}, {Key: "is_completed", Value: false}}
	var order entity.PaymentOrder
	if err := collection.FindOne(ctx, filter).Decode(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (m *MongoDB) GetPaymentOrder(ctx context.Context, id int) (*entity.PaymentOrder, error) {
	return findOne[entity.PaymentOrder](m, ctx, collectionPaymentOrders, bson.D{{Key: "order", Value: id}})
}

// GetDefaultPaymentMethod returns the default usable payment method for a user.
// Only methods with fail_count == 0 are considered. Prefers is_default == true,
// falls back to any usable method.
func (m *MongoDB) GetDefaultPaymentMethod(ctx context.Context, userId string) (*entity.PaymentMethod, error) {
	collection := m.col(collectionPaymentMethods)

	// Prefer the default method, but only if usable.
	filter := bson.D{
		{Key: "user_id", Value: userId},
		{Key: "is_default", Value: true},
		{Key: "fail_count", Value: bson.D{{Key: "$lte", Value: 0}}},
	}
	var pm entity.PaymentMethod
	if err := collection.FindOne(ctx, filter).Decode(&pm); err == nil {
		return &pm, nil
	}

	// Fallback: any usable method for this user.
	filter = bson.D{
		{Key: "user_id", Value: userId},
		{Key: "fail_count", Value: bson.D{{Key: "$lte", Value: 0}}},
	}
	if err := collection.FindOne(ctx, filter).Decode(&pm); err != nil {
		return nil, m.findError(err)
	}
	return &pm, nil
}

// GetPaymentMethodByIdentifier returns a payment method by its card token identifier.
func (m *MongoDB) GetPaymentMethodByIdentifier(ctx context.Context, identifier string) (*entity.PaymentMethod, error) {
	return findOne[entity.PaymentMethod](m, ctx, collectionPaymentMethods, bson.D{{Key: "identifier", Value: identifier}})
}

// UpdatePaymentMethodFailCount updates the fail_count for a payment method.
func (m *MongoDB) UpdatePaymentMethodFailCount(ctx context.Context, identifier string, count int) error {
	collection := m.col(collectionPaymentMethods)
	filter := bson.D{{Key: "identifier", Value: identifier}}
	update := bson.M{"$set": bson.D{{Key: "fail_count", Value: count}}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

// SavePaymentOrder insert or update order
func (m *MongoDB) SavePaymentOrder(ctx context.Context, order *entity.PaymentOrder) error {
	filter := bson.D{{Key: "order", Value: order.Order}}
	set := bson.M{"$set": order}
	collection := m.col(collectionPaymentOrders)
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
			{Key: "$match", Value: bson.M{
				"roaming": true,
			}},
		},
		bson.D{
			{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: collectionChargePoints},
				{Key: "localField", Value: "id"},
				{Key: "foreignField", Value: "location_id"},
				{Key: "as", Value: "charge_points"},
			},
			},
		},
		bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$charge_points"}}}},
		bson.D{
			{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: collectionConnectors},
				{Key: "localField", Value: collectionChargePoints + ".charge_point_id"},
				{Key: "foreignField", Value: "charge_point_id"},
				{Key: "as", Value: "charge_points.connectors"},
			},
			},
		},
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$id"},
				{Key: "root", Value: bson.D{{Key: "$mergeObjects", Value: "$$ROOT"}}},
				{Key: "charge_points", Value: bson.D{{Key: "$push", Value: "$charge_points"}}},
			},
			},
		},
		bson.D{
			{Key: "$replaceRoot", Value: bson.D{
				{Key: "newRoot", Value: bson.D{
					{Key: "$mergeObjects", Value: bson.A{
						"$root",
						bson.D{{Key: "charge_points", Value: "$charge_points"}},
					},
					},
				},
				},
			},
			},
		},
	}
	return aggregateMany[*entity.Location](m, ctx, collectionLocations, pipeline)
}

// SavePreauthorization saves a preauthorization record (insert or update)
func (m *MongoDB) SavePreauthorization(ctx context.Context, preauth *entity.Preauthorization) error {
	collection := m.col(collectionPreauthorizations)
	filter := bson.D{{Key: "order_number", Value: preauth.OrderNumber}}
	set := bson.M{"$set": preauth}
	_, err := collection.UpdateOne(ctx, filter, set, options.Update().SetUpsert(true))
	return err
}

// GetPreauthorization retrieves a preauthorization by order number
func (m *MongoDB) GetPreauthorization(ctx context.Context, orderNumber string) (*entity.Preauthorization, error) {
	return findOne[entity.Preauthorization](m, ctx, collectionPreauthorizations, bson.D{{Key: "order_number", Value: orderNumber}})
}

// GetPreauthorizationByTransaction retrieves a preauthorization by transaction ID
func (m *MongoDB) GetPreauthorizationByTransaction(ctx context.Context, transactionId int) (*entity.Preauthorization, error) {
	collection := m.col(collectionPreauthorizations)
	filter := bson.D{{Key: "transaction_id", Value: transactionId}}
	var preauth entity.Preauthorization
	if err := collection.FindOne(ctx, filter, options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})).Decode(&preauth); err != nil {
		return nil, m.findError(err)
	}
	return &preauth, nil
}

// UpdatePreauthorization updates an existing preauthorization
func (m *MongoDB) UpdatePreauthorization(ctx context.Context, preauth *entity.Preauthorization) error {
	filter := bson.D{{Key: "order_number", Value: preauth.OrderNumber}}
	update := bson.M{"$set": bson.D{
		{Key: "authorization_code", Value: preauth.AuthorizationCode},
		{Key: "captured_amount", Value: preauth.CapturedAmount},
		{Key: "status", Value: preauth.Status},
		{Key: "error_code", Value: preauth.ErrorCode},
		{Key: "error_message", Value: preauth.ErrorMessage},
		{Key: "merchant_data", Value: preauth.MerchantData},
		{Key: "updated_at", Value: preauth.UpdatedAt},
	}}
	return m.updateOne(ctx, collectionPreauthorizations, filter, update, "preauthorization")
}

// GetLastPreauthorizationOrder retrieves the most recent preauthorization order
func (m *MongoDB) GetLastPreauthorizationOrder(ctx context.Context) (*entity.Preauthorization, error) {
	collection := m.col(collectionPreauthorizations)
	filter := bson.D{}
	var preauth entity.Preauthorization
	if err := collection.FindOne(ctx, filter, options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})).Decode(&preauth); err != nil {
		return nil, m.findError(err)
	}
	return &preauth, nil
}

// GetUnbilledTransactions returns all finished transactions where payment_billed < payment_amount
func (m *MongoDB) GetUnbilledTransactions(ctx context.Context) ([]*entity.Transaction, error) {
	filter := bson.D{
		{Key: "is_finished", Value: true},
		{Key: "payment_amount", Value: bson.D{{Key: "$gt", Value: 0}}},
		{Key: "$expr", Value: bson.D{{Key: "$lt", Value: bson.A{"$payment_billed", "$payment_amount"}}}},
	}
	return findMany[*entity.Transaction](m, ctx, collectionTransactions, filter)
}

// SavePaymentRetry upserts a payment retry record by transaction_id
func (m *MongoDB) SavePaymentRetry(ctx context.Context, retry *entity.PaymentRetry) error {
	collection := m.col(collectionPaymentRetries)
	filter := bson.D{{Key: "transaction_id", Value: retry.TransactionId}}
	update := bson.M{"$set": retry}
	_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

// GetPaymentRetry retrieves a payment retry record by transaction_id
func (m *MongoDB) GetPaymentRetry(ctx context.Context, transactionId int) (*entity.PaymentRetry, error) {
	return findOne[entity.PaymentRetry](m, ctx, collectionPaymentRetries, bson.D{{Key: "transaction_id", Value: transactionId}})
}

// GetPendingRetries returns all retry records where next_retry_time <= now
func (m *MongoDB) GetPendingRetries(ctx context.Context, now time.Time) ([]*entity.PaymentRetry, error) {
	filter := bson.D{{Key: "next_retry_time", Value: bson.D{{Key: "$lte", Value: now}}}}
	opts := options.Find().SetSort(bson.D{{Key: "next_retry_time", Value: 1}})
	return findMany[*entity.PaymentRetry](m, ctx, collectionPaymentRetries, filter, opts)
}

// GetAllPaymentRetries returns every payment retry record sorted by next retry time
func (m *MongoDB) GetAllPaymentRetries(ctx context.Context) ([]*entity.PaymentRetry, error) {
	opts := options.Find().SetSort(bson.D{{Key: "next_retry_time", Value: 1}})
	return findMany[*entity.PaymentRetry](m, ctx, collectionPaymentRetries, bson.M{}, opts)
}

// DeletePaymentRetry removes a payment retry record by transaction_id
func (m *MongoDB) DeletePaymentRetry(ctx context.Context, transactionId int) error {
	collection := m.col(collectionPaymentRetries)
	_, err := collection.DeleteOne(ctx, bson.D{{Key: "transaction_id", Value: transactionId}})
	return err
}

// ListMailSubscriptions returns all subscriptions ordered by creation time.
func (m *MongoDB) ListMailSubscriptions(ctx context.Context) ([]*entity.MailSubscription, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	return findMany[*entity.MailSubscription](m, ctx, collectionMailSubscriptions, bson.M{}, opts)
}

// ListMailSubscriptionsByPeriod returns enabled subscriptions matching the period.
func (m *MongoDB) ListMailSubscriptionsByPeriod(ctx context.Context, period string) ([]*entity.MailSubscription, error) {
	filter := bson.D{
		{Key: "period", Value: period},
		{Key: "enabled", Value: true},
	}
	return findMany[*entity.MailSubscription](m, ctx, collectionMailSubscriptions, filter)
}

// GetMailSubscription returns one subscription by id.
func (m *MongoDB) GetMailSubscription(ctx context.Context, id string) (*entity.MailSubscription, error) {
	return findOne[entity.MailSubscription](m, ctx, collectionMailSubscriptions, bson.D{{Key: "_id", Value: id}})
}

// SaveMailSubscription inserts a new subscription or updates an existing one.
func (m *MongoDB) SaveMailSubscription(ctx context.Context, sub *entity.MailSubscription) (*entity.MailSubscription, error) {
	collection := m.col(collectionMailSubscriptions)
	now := time.Now().UTC()
	sub.UpdatedAt = now
	if sub.Id == "" {
		sub.Id = primitive.NewObjectID().Hex()
		sub.CreatedAt = now
		if _, err := collection.InsertOne(ctx, sub); err != nil {
			return nil, err
		}
		return sub, nil
	}
	update := bson.M{"$set": bson.M{
		"email":      sub.Email,
		"period":     sub.Period,
		"user_group": sub.UserGroup,
		"enabled":    sub.Enabled,
		"updated_at": sub.UpdatedAt,
	}}
	if err := m.updateOne(ctx, collectionMailSubscriptions, bson.D{{Key: "_id", Value: sub.Id}}, update, "subscription"); err != nil {
		return nil, err
	}
	return sub, nil
}

// DeleteMailSubscription removes a subscription by id.
func (m *MongoDB) DeleteMailSubscription(ctx context.Context, id string) error {
	return m.deleteOne(ctx, collectionMailSubscriptions, bson.D{{Key: "_id", Value: id}}, "subscription")
}
