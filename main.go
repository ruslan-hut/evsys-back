package main

import (
	"context"
	"evsys-back/config"
	"evsys-back/impl/authenticator"
	"evsys-back/impl/brevo"
	"evsys-back/impl/central-system"
	"evsys-back/impl/core"
	"evsys-back/impl/database"
	databasemock "evsys-back/impl/database-mock"
	"evsys-back/impl/mail"
	"evsys-back/impl/oauth"
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
			log.Error("mongo client", sl.Err(err))
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
			FormUrl:      conf.Redsys.FormUrl,
			NotifyUrl:    conf.Redsys.NotifyUrl,
			Currency:     conf.Redsys.Currency,
		}, log)
		coreHandler.SetRedsys(redsysClient)
		coreHandler.SetCurrency(conf.Redsys.Currency)
		if conf.Redsys.DisablePayment {
			log.Warn("payment processing disabled (test mode)")
			coreHandler.SetDisablePayment(true)
		}
		coreHandler.StartPaymentProcessor()
	}

	var mailService *mail.Service
	if conf.Brevo.Enabled {
		log.With(
			slog.String("sender", conf.Brevo.SenderMail),
			sl.Secret("api_key", conf.Brevo.ApiKey),
		).Info("initializing brevo mail client")
		brevoClient := brevo.New(brevo.Config{
			ApiKey:     conf.Brevo.ApiKey,
			SenderName: conf.Brevo.SenderName,
			SenderMail: conf.Brevo.SenderMail,
			ApiUrl:     conf.Brevo.ApiUrl,
		}, log)
		var mailRepo mail.Repository
		if conf.Mongo.Enabled {
			mailRepo = mongo
		} else {
			mailRepo = mockDb
		}
		mailService = mail.New(mailRepo, rep, brevoClient, log)
		coreHandler.SetMailService(mailService)
		mailService.Start()
	}

	// MCP endpoint and the OAuth server guarding it; the interface stays nil
	// when disabled, which keeps the routes out
	var oauthService http.OAuth
	if conf.Mcp.Enabled {
		var oauthRepo oauth.Repository
		if conf.Mongo.Enabled {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err = mongo.EnsureOAuthIndexes(ctx)
			cancel()
			if err != nil {
				log.Error("oauth indexes", sl.Err(err))
				return
			}
			oauthRepo = mongo
		} else {
			oauthRepo = mockDb
		}
		service, err := oauth.New(oauth.Config{
			PublicUrl:       conf.Mcp.PublicUrl,
			FrontendUrl:     conf.Mcp.FrontendUrl,
			AccessTokenTTL:  time.Duration(conf.Mcp.AccessTokenTTL) * time.Minute,
			RefreshTokenTTL: time.Duration(conf.Mcp.RefreshTokenTTL) * 24 * time.Hour,
		}, oauthRepo, log)
		if err != nil {
			log.Error("mcp oauth", sl.Err(err))
			return
		}
		log.With(
			slog.String("endpoint", service.Resource()),
			slog.String("issuer", service.Issuer()),
			slog.String("consent", conf.Mcp.FrontendUrl),
		).Info("mcp server enabled")
		oauthService = service
	}

	server := http.NewServer(conf, log, coreHandler, oauthService)
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

	// Stop payment processor
	coreHandler.StopPaymentProcessor()

	// Stop mail scheduler
	if mailService != nil {
		mailService.Stop()
	}

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
