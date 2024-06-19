package logger

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"
)

const (
	envLocal    = "local"
	envDev      = "dev"
	envProd     = "prod"
	logFileName = "evsys-back.log"
)

type LogEntry struct {
	Time  time.Time `json:"time"`
	Level string    `json:"level"`
}

func SetupLogger(env, path string) *slog.Logger {
	var logger *slog.Logger
	var logFile *os.File
	var err error

	if env != envLocal {
		logPath := logFilePath(path)
		logFile, err = os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal("error opening log file: ", err)
		}
		log.Printf("env: %s; log file: %s", env, logPath)
	}

	switch env {
	case envLocal:
		logger = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envDev:
		logger = slog.New(
			slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		logger = slog.New(
			slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return logger
}

func logFilePath(path string) string {
	return fmt.Sprintf("%s/%s", path, logFileName)
}
