package internal

import (
	"encoding/json"
	"evsys-back/services"
	"fmt"
)

type CallType string

const (
	ReadSysLog  CallType = "ReadSysLog"
	ReadBackLog CallType = "ReadBackLog"
)

type Call struct {
	CallType CallType
	Remote   string
}

type Handler struct {
	logger   services.LogHandler
	database services.Database
}

func (h *Handler) SetLogger(logger services.LogHandler) {
	h.logger = logger
}

func (h *Handler) SetDatabase(database services.Database) {
	h.database = database
}

func NewApiHandler() *Handler {
	handler := Handler{}
	return &handler
}

func (h *Handler) HandleApiCall(ac *Call) []byte {
	h.logger.Info(fmt.Sprintf("call %s from remote %s", ac.CallType, ac.Remote))
	if h.database == nil {
		return nil
	}

	var data interface{}
	var err error

	switch ac.CallType {
	case ReadSysLog:
		data, err = h.database.ReadSystemLog()
	case ReadBackLog:
		data, err = h.database.ReadBackLog()
	default:
		h.logger.Warn("unknown call type")
		return nil
	}

	if err != nil {
		h.logger.Error("read database", err)
		return nil
	}
	byteData, err := json.Marshal(data)
	if err != nil {
		h.logger.Error("encoding data", err)
		return nil
	}
	return byteData
}
