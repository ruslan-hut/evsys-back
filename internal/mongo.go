package internal

import (
	"context"
	"evsys-back/config"
	"evsys-back/models"
	"evsys-back/services"
	"evsys-back/utility"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"strings"
	"time"
)

const (
	collectionSysLog       = "sys_log"
	collectionBackLog      = "back_log"
	collectionUsers        = "users"
	collectionUserTags     = "user_tags"
	collectionChargePoints = "charge_points"
	collectionConnectors   = "connectors"
	collectionTransactions = "transactions"
	collectionMeterValues  = "meter_values"
)

type pipeResult struct {
	Id   string    `bson:"_id"`
	Info string    `bson:"info"`
	Time time.Time `bson:"time"`
}

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

func (m *MongoDB) Write(table string, data services.Data) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)
	collection := connection.Database(m.database).Collection(table)
	_, err = collection.InsertOne(m.ctx, data)
	return err
}

func (m *MongoDB) WriteLogMessage(data services.Data) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)
	collection := connection.Database(m.database).Collection(collectionSysLog)
	_, err = collection.InsertOne(m.ctx, data)
	return err
}

func (m *MongoDB) ReadSystemLog() (interface{}, error) {
	return m.read(collectionSysLog, services.FeatureMessageType)
}

func (m *MongoDB) ReadBackLog() (interface{}, error) {
	return m.read(collectionBackLog, services.LogMessageType)
}

func (m *MongoDB) GetUser(username string) (*models.User, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)
	collection := connection.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"username", username}}
	var userData models.User
	err = collection.FindOne(m.ctx, filter).Decode(&userData)
	if err != nil {
		return nil, err
	}
	return &userData, nil
}

func (m *MongoDB) GetUserById(userId string) (*models.User, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"user_id", userId}}
	var userData models.User
	err = collection.FindOne(m.ctx, filter).Decode(&userData)
	if err != nil {
		return nil, err
	}
	return &userData, nil
}

func (m *MongoDB) GetUserTags(userId string) ([]models.UserTag, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUserTags)
	filter := bson.D{{"user_id", userId}}
	var userTags []models.UserTag
	cursor, err := collection.Find(m.ctx, filter)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(m.ctx, &userTags); err != nil {
		return nil, err
	}
	return userTags, nil
}

func (m *MongoDB) GetUserTag(idTag string) (*models.UserTag, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUserTags)
	filter := bson.D{{"id_tag", idTag}}
	var userTag models.UserTag
	if err = collection.FindOne(m.ctx, filter).Decode(&userTag); err != nil {
		return nil, err
	}
	return &userTag, nil
}

func (m *MongoDB) AddUserTag(userTag *models.UserTag) error {
	existedUserTag, err := m.GetUserTag(userTag.IdTag)
	if err == nil {
		return fmt.Errorf("user tag %s already exists for user %s", existedUserTag.IdTag, existedUserTag.UserId)
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

func (m *MongoDB) UpdateUser(user *models.User) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"username", user.Username}}
	update := bson.M{"$set": user}
	_, err = collection.UpdateOne(m.ctx, filter, update)
	return err
}

func (m *MongoDB) AddUser(user *models.User) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUsers)
	_, err = collection.InsertOne(m.ctx, user)
	return err
}

func (m *MongoDB) CheckToken(token string) (*models.User, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"token", token}}
	var userData models.User
	err = collection.FindOne(m.ctx, filter).Decode(&userData)
	if err != nil {
		return nil, err
	}
	return &userData, nil
}

func (m *MongoDB) read(table, dataType string) (interface{}, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	var logMessages interface{}
	switch dataType {
	case services.FeatureMessageType:
		logMessages = []services.FeatureMessage{}
	case services.LogMessageType:
		logMessages = []services.LogMessage{}
	default:
		return nil, fmt.Errorf("unknown data type: %s", dataType)
	}

	collection := connection.Database(m.database).Collection(table)
	filter := bson.D{}
	opts := options.Find().SetSort(bson.D{{"timestamp", -1}})
	if m.logRecordsNumber > 0 {
		opts.SetLimit(m.logRecordsNumber)
	}
	cursor, err := collection.Find(m.ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(m.ctx, &logMessages); err != nil {
		return nil, err
	}
	return logMessages, nil
}

func (m *MongoDB) GetChargePoints() (interface{}, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	pipeline := mongo.Pipeline{
		{{"$lookup", bson.M{
			"from":         "connectors",
			"localField":   "charge_point_id",
			"foreignField": "charge_point_id",
			"as":           "Connectors",
		}}},
		bson.D{{"$sort", bson.D{{"charge_point_id", 1}}}},
	}

	collection := connection.Database(m.database).Collection(collectionChargePoints)
	var chargePoints []models.ChargePoint
	cursor, err := collection.Aggregate(m.ctx, pipeline)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(m.ctx, &chargePoints); err != nil {
		return nil, err
	}

	// adding last Heartbeat event time
	logPipe := bson.A{
		bson.D{{"$match", bson.D{{"feature", "Heartbeat"}}}},
		bson.D{{"$sort", bson.D{{"time", -1}}}},
		bson.D{
			{"$group",
				bson.D{
					{"_id", "$charge_point_id"},
					{"time", bson.D{{"$first", "$timestamp"}}},
				},
			},
		},
		bson.D{{"$sort", bson.D{{"_id", 1}}}},
	}

	var pipeResult []pipeResult
	cLog := connection.Database(m.database).Collection(collectionSysLog)
	cursor, err = cLog.Aggregate(m.ctx, logPipe)
	if err != nil {
		return nil, fmt.Errorf("aggregate Heartbeat: %v", err)
	}
	if err = cursor.All(m.ctx, &pipeResult); err != nil {
		return nil, fmt.Errorf("decode Heartbeat: %v", err)
	}

	for _, heartbeat := range pipeResult {
		for i, chp := range chargePoints {
			if chp.Id == heartbeat.Id {
				chargePoints[i].LastSeen = utility.TimeAgo(heartbeat.Time)
				break
			}
		}
	}

	return chargePoints, nil
}

func (m *MongoDB) GetChargePoint(id string) (*models.ChargePoint, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionChargePoints)
	filter := bson.D{{"charge_point_id", id}}
	var chargePoint models.ChargePoint
	if err = collection.FindOne(m.ctx, filter).Decode(&chargePoint); err != nil {
		return nil, err
	}
	return &chargePoint, nil
}

// GetConnector get single connector by charge point id and connector id
func (m *MongoDB) GetConnector(chargePointId string, connectorId int) (*models.Connector, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionConnectors)
	filter := bson.D{{"charge_point_id", chargePointId}, {"connector_id", connectorId}}
	var connector models.Connector
	if err = collection.FindOne(m.ctx, filter).Decode(&connector); err != nil {
		return nil, err
	}
	return &connector, nil
}

func (m *MongoDB) getTransactionState(transaction *models.Transaction) (*models.ChargeState, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	var chargeState models.ChargeState

	chargePoint, err := m.GetChargePoint(transaction.ChargePointId)
	if err != nil {
		return nil, fmt.Errorf("get charge point: %v", err)
	}
	connector, err := m.GetConnector(transaction.ChargePointId, transaction.ConnectorId)
	if err != nil {
		return nil, fmt.Errorf("get connector: %v", err)
	}
	consumed := 0
	meterValue, _ := m.GetLastMeterValue(transaction.TransactionId)
	if meterValue != nil {
		consumed = meterValue.Value - transaction.MeterStart
	}
	chargeState = models.ChargeState{
		TransactionId:      transaction.TransactionId,
		ConnectorId:        transaction.ConnectorId,
		Connector:          "",
		ChargePointId:      transaction.ChargePointId,
		ChargePointTitle:   chargePoint.Title,
		ChargePointAddress: chargePoint.Address,
		TimeStarted:        transaction.TimeStart,
		Duration:           int(time.Since(transaction.TimeStart).Seconds()),
		MeterStart:         transaction.MeterStart,
		Consumed:           consumed,
		Status:             connector.Status,
		IsCharging:         transaction.IsFinished == false,
		CanStop:            false,
	}

	return &chargeState, nil
}

func (m *MongoDB) GetTransaction(id int) (*models.ChargeState, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionTransactions)
	filter := bson.D{{"transaction_id", id}}
	var transaction models.Transaction
	if err = collection.FindOne(m.ctx, filter).Decode(&transaction); err != nil {
		return nil, err
	}

	chargeState, err := m.getTransactionState(&transaction)
	if err != nil {
		return nil, err
	}
	return chargeState, nil
}

func (m *MongoDB) GetActiveTransactions(userId string) ([]*models.ChargeState, error) {
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
		{"is_finished", false},
	}
	var transactions []models.Transaction
	cursor, err := collection.Find(m.ctx, filter)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(m.ctx, &transactions); err != nil {
		return nil, err
	}

	var chargeStates []*models.ChargeState
	for _, transaction := range transactions {
		chargeState, err := m.getTransactionState(&transaction)
		if err != nil {
			return nil, err
		}
		chargeState.CanStop = true
		chargeStates = append(chargeStates, chargeState)
	}
	return chargeStates, nil
}

func (m *MongoDB) GetTransactionByTag(idTag string, timeStart time.Time) (*models.Transaction, error) {
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
	var transaction models.Transaction
	if err = collection.FindOne(m.ctx, filter).Decode(&transaction); err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (m *MongoDB) GetLastMeterValue(transactionId int) (*models.TransactionMeter, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionMeterValues)
	filter := bson.D{
		{"transaction_id", transactionId},
	}
	var value models.TransactionMeter
	if err = collection.FindOne(m.ctx, filter, options.FindOne().SetSort(bson.D{{"time", -1}})).Decode(&value); err != nil {
		return nil, err
	}
	return &value, nil
}
