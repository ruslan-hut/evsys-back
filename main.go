package main

import (
	"evsys-back/config"
	"evsys-back/internal"
	"evsys-back/internal/api/http"
	"evsys-back/internal/lib/logger"
	"evsys-back/internal/lib/sl"
	"evsys-back/services"
	"flag"
)

func main() {

	configPath := flag.String("conf", "config.yml", "path to config file")
	logPath := flag.String("log", "/var/log/wattbrews", "path to log file directory")
	flag.Parse()

	conf := config.GetConfig(*configPath)
	log := logger.SetupLogger(conf.Env, *logPath)

	var mongo services.Database
	var err error
	if conf.Mongo.Enabled {
		mongo, err = internal.NewMongoClient(conf)
		if err != nil {
			log.Error("mongo client failed", sl.Err(err))
			return
		}
	}

	var cs services.CentralSystemService
	if conf.CentralSystem.Enabled {
		cs = internal.NewCentralSystem(conf.CentralSystem.Url, conf.CentralSystem.Token)
		log.Info("central system initialized")
	}

	auth := internal.NewAuthenticator()
	auth.SetLogger(internal.NewLogger("auth", false, mongo))
	auth.SetDatabase(mongo)

	var firebase *internal.Firebase
	if conf.FirebaseKey != "" {
		firebase, err = internal.NewFirebase(conf.FirebaseKey)
		if err != nil {
			log.Error("firebase client", err)
			return
		}
		firebase.SetLogger(internal.NewLogger("firebase", false, mongo))
		auth.SetFirebase(firebase)
	}

	apiHandler := internal.NewApiHandler()
	apiHandler.SetLogger(internal.NewLogger("api", false, mongo))
	apiHandler.SetDatabase(mongo)
	apiHandler.SetCentralSystem(cs)
	apiHandler.SetAuth(auth)

	statusReader := internal.NewStatusReader()
	statusReader.SetLogger(internal.NewLogger("status", false, mongo))
	statusReader.SetDatabase(mongo)

	payments := internal.NewPayments()
	payments.SetLogger(internal.NewLogger("payments", false, mongo))
	payments.SetDatabase(mongo)

	server := http.NewServer(conf, log, nil)
	server.SetApiHandler(apiHandler.HandleApiCall)
	server.SetWsHandler(apiHandler.HandleUserRequest)
	server.SetAuth(auth)
	server.SetStatusReader(statusReader)
	server.SetPaymentsService(payments)

	err = server.Start()
	if err != nil {
		log.Error("server start", err)
		return
	}

}
