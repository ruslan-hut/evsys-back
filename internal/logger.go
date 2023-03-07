package internal

import (
	"evsys-back/services"
	"fmt"
	"log"
	"time"
)

type Importance string

const (
	Info    Importance = " "
	Warning Importance = "?"
	Error   Importance = "!"
	Raw     Importance = "-"
)

type Logger struct {
	messageService services.MessageService
	database       services.Database
	debugMode      bool
	category       string
}

func NewLogger(category string) *Logger {
	return &Logger{
		debugMode: false,
		category:  category,
	}
}

func (l *Logger) SetDebugMode(debugMode bool) {
	l.debugMode = debugMode
}

func (l *Logger) SetMessageService(messageService services.MessageService) {
	l.messageService = messageService
}

func (l *Logger) SetDatabase(database services.Database) {
	l.database = database
}

func logTime(t time.Time) string {
	timeString := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	return timeString
}
func (l *Logger) Info(text string) {
	l.logEvent(Info, text)
}

func (l *Logger) Debug(text string) {
	l.logEvent(Raw, text)
}

func (l *Logger) Warn(text string) {
	l.logEvent(Warning, text)
}

func (l *Logger) Error(event string, err error) {
	text := fmt.Sprintf("%s: %s", event, err.Error())
	l.logEvent(Error, text)
}

func (l *Logger) internalError(event string, err error) {
	text := fmt.Sprintf("  %s logger: %s: %s", l.category, event, err.Error())
	l.logLine(string(Error), text)
}

func (l *Logger) logEvent(level Importance, text string) {

	message := &services.LogMessage{
		Time:     logTime(time.Now()),
		Text:     text,
		Category: l.category,
		Level:    string(level),
	}

	messageText := fmt.Sprintf("%s: %s", message.Category, message.Text)
	l.logLine(message.Level, messageText)

	if l.messageService != nil {
		if err := l.messageService.Send(message); err != nil {
			l.internalError("sending message", err)
		}
	}

	if l.database != nil {
		if err := l.database.WriteLogMessage(message); err != nil {
			l.internalError("write to database", err)
		}
	}
}

func (l *Logger) logLine(importance, text string) {
	if importance == string(Raw) {
		return
	}
	if importance == string(Info) && !l.debugMode && l.database != nil {
		return
	}
	log.Printf("%s %s", importance, text)
}
