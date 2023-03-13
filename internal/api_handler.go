package internal

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"evsys-back/models"
	"evsys-back/services"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"net/http"
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
	Token    string
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

func (h *Handler) HandleApiCall(ac *Call) ([]byte, int) {
	h.logger.Info(fmt.Sprintf("call %s from remote %s", ac.CallType, ac.Remote))
	if h.database == nil {
		return nil, http.StatusOK
	}

	var data interface{}
	var err error
	status := http.StatusOK

	if ac.CallType != AuthenticateUser {
		if err = h.checkToken(ac.Token); err != nil {
			h.logger.Info("token: " + ac.Token)
			h.logger.Error("invalid token", err)
			status = http.StatusUnauthorized
			return nil, status
		}
	}

	switch ac.CallType {
	case ReadSysLog:
		data, err = h.database.ReadSystemLog()
		if err != nil {
			h.logger.Error("read system log", err)
			status = http.StatusInternalServerError
		}
	case ReadBackLog:
		data, err = h.database.ReadBackLog()
		if err != nil {
			h.logger.Error("read back log", err)
			status = http.StatusInternalServerError
		}
	case AuthenticateUser:
		var user models.User
		err = json.Unmarshal(ac.Payload, &user)
		if err != nil {
			h.logger.Error("decoding user", err)
			status = http.StatusUnsupportedMediaType
		} else {
			data, err = h.authenticateUser(user.Username, user.Password)
			if err != nil {
				h.logger.Error("user authentication", err)
				status = http.StatusUnauthorized
			}
		}
	default:
		h.logger.Warn("unknown call type")
		return nil, http.StatusBadRequest
	}

	if err != nil {
		return nil, status
	}
	byteData, err := json.Marshal(data)
	if err != nil {
		h.logger.Error("encoding data", err)
		return nil, http.StatusInternalServerError
	}
	return byteData, status
}

func (h *Handler) authenticateUser(username, password string) (*models.User, error) {
	userData, err := h.database.GetUser(username)
	if err != nil {
		return nil, err
	}

	// temporary solution
	//if username == "user" && password == "pass" {
	//	userData.Token = h.generateToken()
	//	userData.Password = h.generatePasswordHash(password)
	//	err = h.database.UpdateUser(userData)
	//	if err != nil {
	//		return nil, err
	//	}
	//	userData.Password = ""
	//	return userData, nil
	//}

	result := bcrypt.CompareHashAndPassword([]byte(userData.Password), []byte(password))
	if result == nil {
		token := h.generateToken()
		userData.Token = token
		err = h.database.UpdateUser(userData)
		if err != nil {
			return nil, err
		}
	}
	userData.Password = ""
	h.logger.Info("user authorized: " + username)
	return userData, nil
}

func (h *Handler) checkToken(token string) error {
	return h.database.CheckToken(token)
}

func (h *Handler) generatePasswordHash(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("generating password hash", err)
		return ""
	}
	return string(hash)
}

func (h *Handler) generateToken() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
