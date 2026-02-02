package main

import (
	"context"
	"evsys-back/config"
	"evsys-back/impl/authenticator"
	"evsys-back/impl/central-system"
	"evsys-back/impl/core"
	"evsys-back/impl/database"
	databasemock "evsys-back/impl/database-mock"
	"evsys-back/impl/redsys"
	"evsys-back/impl/reports"
	statusreader "evsys-back/impl/status-reader"
	"evsys-back/internal/api/http"
	"evsys-back/internal/firebase"
	"evsys-back/internal/lib/logger"
	"evsys-back/internal/lib/sl"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var mongo *database.MongoDB
var mockDb *databasemock.MockDB

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
			log.Error("mongo client", err)
			return
		}
	} else {
		log.Info("using mock db")
		mockDb = databasemock.NewMockDB()
	}

	var auth *authenticator.Authenticator
	if conf.Mongo.Enabled {
		auth = authenticator.New(log, mongo)
	} else {
		auth = authenticator.New(log, mockDb)
	}

	var rep *reports.Reports
	if conf.Mongo.Enabled {
		rep = reports.New(mongo, log)
	} else {
		rep = reports.New(mockDb, log)
	}

	var fb *firebase.Firebase
	if conf.FirebaseKey != "" {
		log.Info("firebase enabled")
		fb, err = firebase.New(conf.FirebaseKey)
		if err != nil {
			log.Error("firebase client", err)
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
	coreHandler.SetReports(rep)

	if conf.CentralSystem.Enabled {
		log.With(
			slog.String("url", conf.CentralSystem.Url),
			sl.Secret("token", conf.CentralSystem.Token),
		).Info("connecting to central system")
		cs := centralsystem.NewCentralSystem(conf.CentralSystem.Url, conf.CentralSystem.Token)
		coreHandler.SetCentralSystem(cs)
	}

	if conf.Redsys.Enabled {
		log.With(
			slog.String("merchant_code", conf.Redsys.MerchantCode),
			slog.String("terminal", conf.Redsys.Terminal),
		).Info("initializing redsys client")
		redsysClient := redsys.NewClient(redsys.Config{
			MerchantCode: conf.Redsys.MerchantCode,
			Terminal:     conf.Redsys.Terminal,
			SecretKey:    conf.Redsys.SecretKey,
			RestApiUrl:   conf.Redsys.RestApiUrl,
			Currency:     conf.Redsys.Currency,
		}, log)
		coreHandler.SetRedsys(redsys.NewAdapter(redsysClient))
	}

	server := http.NewServer(conf, log, coreHandler)
	if conf.Mongo.Enabled {
		server.SetStatusReader(statusreader.New(log, mongo))
	}

	// Graceful shutdown setup
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Error("server start", sl.Err(err))
		}
	}()

	log.Info("server started", slog.String("port", conf.Listen.Port))

	// Wait for shutdown signal
	<-shutdown
	log.Info("shutting down...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		log.Error("server shutdown", sl.Err(err))
	}

	// Close MongoDB connection
	if mongo != nil {
		if err := mongo.Close(); err != nil {
			log.Error("mongodb close", sl.Err(err))
		} else {
			log.Info("mongodb connection closed")
		}
	}

	log.Info("shutdown complete")
}
