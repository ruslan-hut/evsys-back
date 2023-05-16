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

	var cs services.CentralSystemService
	if conf.CentralSystem.Enabled {
		cs = internal.NewCentralSystem(conf.CentralSystem.Url)
		logger.Info("central system initialized")
	} else {
		logger.Info("central system is disabled")
	}

	apiLogger := internal.NewLogger("api")
	apiLogger.SetDebugMode(conf.IsDebug)

	api := internal.NewApiHandler()
	api.SetLogger(apiLogger)
	api.SetDatabase(mongo)
	api.SetCentralSystem(cs)

	if conf.FirebaseKey != "" {
		firebase, err := internal.NewFirebase(conf.FirebaseKey)
		if err != nil {
			logger.Error("firebase client", err)
			return
		}
		firebase.SetLogger(internal.NewLogger("firebase"))
		api.SetFirebase(firebase)
	}

	serverLogger := internal.NewLogger("server")
	serverLogger.SetDebugMode(conf.IsDebug)

	server := internal.NewServer(conf)
	server.SetLogger(serverLogger)
	server.SetApiHandler(api.HandleApiCall)
	server.SetWsHandler(api.HandleUserRequest)

	err = server.Start()
	if err != nil {
		logger.Error("server start", err)
		return
	}

}
