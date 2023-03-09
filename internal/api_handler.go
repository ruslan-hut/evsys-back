package internal

import (
	"encoding/json"
	"evsys-back/services"
	"fmt"
)

type CallType string

const (
	ReadSysLog       CallType = "ReadSysLog"
	ReadBackLog      CallType = "ReadBackLog"
	AuthenticateUser CallType = "AuthenticateUser"
)

type Call struct {
	CallType CallType
	Remote   string
	Payload  []byte
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
	case AuthenticateUser:
		var user User
		err = json.Unmarshal(ac.Payload, &user)
		if err != nil {
			h.logger.Error("decoding user", err)
		} else {
			data, err = h.database.AuthenticateUser(user.Username, user.Password)
		}
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
