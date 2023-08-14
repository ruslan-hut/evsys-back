package main

import (
	"evsys-back/config"
	"evsys-back/internal"
	"evsys-back/services"
	"flag"
)

func main() {

	logger := internal.NewLogger("internal", false, nil)

	configPath := flag.String("conf", "config.yml", "path to config file")
	flag.Parse()

	logger.Info("using config file: " + *configPath)
	conf, err := config.GetConfig(*configPath)
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

	auth := internal.NewAuthenticator()
	auth.SetLogger(internal.NewLogger("auth", conf.IsDebug, mongo))
	auth.SetDatabase(mongo)

	if conf.FirebaseKey != "" {
		firebase, err := internal.NewFirebase(conf.FirebaseKey)
		if err != nil {
			logger.Error("firebase client", err)
			return
		}
		firebase.SetLogger(internal.NewLogger("firebase", conf.IsDebug, mongo))
		auth.SetFirebase(firebase)
	}

	api := internal.NewApiHandler()
	api.SetLogger(internal.NewLogger("api", conf.IsDebug, mongo))
	api.SetDatabase(mongo)
	api.SetCentralSystem(cs)
	api.SetAuth(auth)

	statusReader := internal.NewStatusReader()
	statusReader.SetLogger(internal.NewLogger("status", conf.IsDebug, mongo))
	statusReader.SetDatabase(mongo)

	payments := internal.NewPayments()
	payments.SetLogger(internal.NewLogger("payments", conf.IsDebug, mongo))
	payments.SetDatabase(mongo)

	server := internal.NewServer(conf)
	server.SetLogger(internal.NewLogger("server", conf.IsDebug, mongo))
	server.SetApiHandler(api.HandleApiCall)
	server.SetWsHandler(api.HandleUserRequest)
	server.SetAuth(auth)
	server.SetStatusReader(statusReader)
	server.SetPaymentsService(payments)

	err = server.Start()
	if err != nil {
		logger.Error("server start", err)
		return
	}

}
