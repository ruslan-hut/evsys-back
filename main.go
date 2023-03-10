package main

import (
	"evsys-back/config"
	"evsys-back/internal"
	"evsys-back/services"
)

func main() {

	logger := internal.NewLogger("internal")

	conf, err := config.GetConfig()
	if err != nil {
		logger.Error("boot", err)
		return
	}

	var mongo services.Database
	if conf.Mongo.Enabled {
		mongo, err = internal.NewMongoClient(conf)
		if err != nil {
			logger.Error("mongo client", err)
			return
		}
		logger.Info("mongo client initialized")
	}

	apiLogger := internal.NewLogger("api")
	apiLogger.SetDebugMode(conf.IsDebug)

	api := internal.NewApiHandler()
	api.SetLogger(apiLogger)
	api.SetDatabase(mongo)

	serverLogger := internal.NewLogger("server")
	serverLogger.SetDebugMode(conf.IsDebug)

	server := internal.NewServer(conf)
	server.SetLogger(serverLogger)
	server.SetApiHandler(api.HandleApiCall)

	err = server.Start()
	if err != nil {
		logger.Error("server start", err)
		return
	}

}
