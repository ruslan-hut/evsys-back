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
	RegisterUser     CallType = "RegisterUser"
	GetChargePoints  CallType = "GetChargePoints"
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
	if h.database == nil {
		h.logger.Info(fmt.Sprintf("call %s from remote %s", ac.CallType, ac.Remote))
		return nil, http.StatusOK
	}

	var data interface{}
	var err error
	status := http.StatusOK

	if ac.CallType != AuthenticateUser && ac.CallType != RegisterUser {
		if err = h.checkToken(ac.Token); err != nil {
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
		userData, err := h.unmarshallUserData(ac.Payload)
		if err != nil {
			h.logger.Error("decoding user", err)
			status = http.StatusUnsupportedMediaType
		} else {
			data, err = h.authenticateUser(userData.Username, userData.Password)
			if err != nil {
				h.logger.Error("user authentication", err)
				status = http.StatusUnauthorized
			}
		}
	case RegisterUser:
		userData, err := h.unmarshallUserData(ac.Payload)
		if err != nil {
			h.logger.Error("decoding user", err)
			status = http.StatusUnsupportedMediaType
		} else {
			err, status = h.registerUser(userData)
			if err != nil {
				h.logger.Error("user registration", err)
			}
		}
	case GetChargePoints:
		data, err = h.database.GetChargePoints()
		if err != nil {
			h.logger.Error("get charge points", err)
			status = http.StatusInternalServerError
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

func (h *Handler) unmarshallUserData(data []byte) (*models.User, error) {
	var user models.User
	err := json.Unmarshal(data, &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (h *Handler) registerUser(user *models.User) (error, int) {
	if user.Password == "" {
		return fmt.Errorf("empty password"), http.StatusBadRequest
	}
	if user.Username == "" {
		return fmt.Errorf("empty username"), http.StatusBadRequest
	}
	existedUser, _ := h.database.GetUser(user.Username)
	if existedUser != nil {
		return nil, http.StatusConflict
	}
	user.Password = h.generatePasswordHash(user.Password)
	if user.Password == "" {
		return fmt.Errorf("empty password hash"), http.StatusInternalServerError
	}
	err := h.database.AddUser(user)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, http.StatusOK
}

func (h *Handler) authenticateUser(username, password string) (*models.User, error) {
	userData, err := h.database.GetUser(username)
	if err != nil {
		return nil, err
	}
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
	if token == "" {
		return fmt.Errorf("empty token")
	}
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
