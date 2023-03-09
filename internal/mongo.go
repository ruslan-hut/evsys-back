package internal

import (
	"context"
	"evsys-back/config"
	"evsys-back/services"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

const collectionSysLog = "sys_log"
const collectionBackLog = "back_log"
const collectionUsers = "users"

type MongoDB struct {
	ctx           context.Context
	clientOptions *options.ClientOptions
	database      string
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
		ctx:           context.Background(),
		clientOptions: clientOptions,
		database:      conf.Mongo.Database,
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
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoDB) WriteLogMessage(data services.Data) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)
	collection := connection.Database(m.database).Collection(collectionSysLog)
	_, err = collection.InsertOne(m.ctx, data)
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoDB) ReadSystemLog() (interface{}, error) {
	return m.read(collectionSysLog, services.FeatureMessageType)
}

func (m *MongoDB) ReadBackLog() (interface{}, error) {
	return m.read(collectionBackLog, services.LogMessageType)
}

func (m *MongoDB) AuthenticateUser(username string, password string) (interface{}, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)
	collection := connection.Database(m.database).Collection(collectionUsers)
	filter := bson.D{{"username", username}, {"password", password}}
	var user User
	err = collection.FindOne(m.ctx, filter).Decode(&user)
	if err != nil {
		return nil, err
	}
	user.Password = ""
	return user, nil
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
	opts := options.Find().SetSort(bson.D{{"time", -1}}).SetLimit(1000)
	cursor, err := collection.Find(m.ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(m.ctx, &logMessages); err != nil {
		return nil, err
	}
	return logMessages, nil
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
		log.Println("mongodb disconnect error;", err)
	}
}
