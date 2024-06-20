package main

import (
	"evsys-back/config"
	"evsys-back/impl/authenticator"
	"evsys-back/impl/central-system"
	"evsys-back/impl/core"
	"evsys-back/impl/database"
	statusreader "evsys-back/impl/status-reader"
	"evsys-back/internal/api/http"
	"evsys-back/internal/firebase"
	"evsys-back/internal/lib/logger"
	"evsys-back/internal/lib/sl"
	"flag"
	"log/slog"
)

var mongo *database.MongoDB
var mockDb *database.MockDB

func main() {

	configPath := flag.String("conf", "config.yml", "path to config file")
	logPath := flag.String("log", "/var/log/wattbrews", "path to log file directory")
	flag.Parse()

	conf := config.GetConfig(*configPath)
	log := logger.SetupLogger(conf.Env, *logPath)

	var err error
	if conf.Mongo.Enabled {
		log.With(
			slog.String("host", conf.Mongo.Host),
			slog.String("db", conf.Mongo.Database),
		).Info("connecting to mongo")
		mongo, err = database.NewMongoClient(conf)
		if err != nil {
			log.Error("mongo client", sl.Err(err))
			return
		}
	} else {
		log.Info("using mock db")
		mockDb = database.NewMockDB()
	}

	var cs *centralsystem.CentralSystem
	if conf.CentralSystem.Enabled {
		log.With(
			slog.String("url", conf.CentralSystem.Url),
			sl.Secret("token", conf.CentralSystem.Token),
		).Info("connecting to central system")
		cs = centralsystem.NewCentralSystem(conf.CentralSystem.Url, conf.CentralSystem.Token)
	}

	var auth *authenticator.Authenticator
	if conf.Mongo.Enabled {
		auth = authenticator.New(log, mongo)
	} else {
		auth = authenticator.New(log, mockDb)
	}

	var fb *firebase.Firebase
	if conf.FirebaseKey != "" {
		log.Info("firebase enabled")
		fb, err = firebase.New(log, conf.FirebaseKey)
		if err != nil {
			log.Error("firebase client", sl.Err(err))
			return
		}
		auth.SetFirebase(fb)
	}

	var coreHandler *core.Core
	if conf.Mongo.Enabled {
		coreHandler = core.New(log, mongo)
	} else {
		coreHandler = core.New(log, mockDb)
	}
	coreHandler.SetAuth(auth)
	if conf.CentralSystem.Enabled {
		coreHandler.SetCentralSystem(cs)
	}

	server := http.NewServer(conf, log, coreHandler)
	if conf.Mongo.Enabled {
		server.SetStatusReader(statusreader.New(log, mongo))
	} else {
		server.SetStatusReader(statusreader.New(log, mockDb))
	}

	err = server.Start()
	if err != nil {
		log.Error("server start", err)
	}
}
