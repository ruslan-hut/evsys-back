package internal

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"evsys-back/models"
	"evsys-back/services"
	"fmt"
	"net/http"
	"strconv"
)

type CallType string

const (
	ReadSysLog           CallType = "ReadSysLog"
	ReadBackLog          CallType = "ReadBackLog"
	AuthenticateUser     CallType = "AuthenticateUser"
	RegisterUser         CallType = "RegisterUser"
	GetChargePoints      CallType = "GetChargePoints"
	CentralSystemCommand CallType = "CentralSystemCommand"
	ActiveTransactions   CallType = "ActiveTransactions"
	TransactionInfo      CallType = "TransactionInfo"
)

type Call struct {
	CallType CallType
	Remote   string
	Payload  []byte
	Token    string
}

type Handler struct {
	logger        services.LogHandler
	database      services.Database
	centralSystem services.CentralSystemService
	auth          services.Auth
}

func (h *Handler) SetLogger(logger services.LogHandler) {
	h.logger = logger
}

func (h *Handler) SetDatabase(database services.Database) {
	h.database = database
}

func (h *Handler) SetCentralSystem(centralSystem services.CentralSystemService) {
	h.centralSystem = centralSystem
}

func (h *Handler) SetAuth(auth services.Auth) {
	h.auth = auth
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

	userId := ""
	var data interface{}
	var err error
	status := http.StatusOK

	if ac.CallType != AuthenticateUser && ac.CallType != RegisterUser {
		if h.auth == nil {
			h.logger.Warn("authenticator not initialized")
			status = http.StatusInternalServerError
			return nil, status
		}
		user, err := h.auth.GetUser(ac.Token)
		if err != nil {
			h.logger.Error("token check failed", err)
			status = http.StatusUnauthorized
			return nil, status
		}
		userId = user.UserId
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
			data, err = h.auth.AuthenticateUser(userData.Username, userData.Password)
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
			err = h.auth.RegisterUser(userData)
			if err != nil {
				h.logger.Error("user registration", err)
				status = http.StatusInternalServerError
			}
		}
	case GetChargePoints:
		data, err = h.database.GetChargePoints()
		if err != nil {
			h.logger.Error("get charge points", err)
			status = http.StatusInternalServerError
		}
	case ActiveTransactions:
		data, err = h.database.GetActiveTransactions(userId)
		if err != nil {
			h.logger.Error("get active transactions", err)
			status = http.StatusInternalServerError
		}
	case TransactionInfo:
		id, err := strconv.Atoi(string(ac.Payload))
		if err != nil {
			h.logger.Error("decoding transaction id", err)
			status = http.StatusBadRequest
		} else {
			data, err = h.database.GetTransaction(id)
			if err != nil {
				h.logger.Error("get transaction", err)
				status = http.StatusInternalServerError
			}
		}
	case CentralSystemCommand:
		if ac.Payload == nil {
			h.logger.Info("empty payload for central system command")
			status = http.StatusBadRequest
		} else {
			data, err = h.handleCentralSystemCommand(ac.Payload)
			if err != nil {
				h.logger.Error("handle central system command", err)
				status = http.StatusInternalServerError
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

func (h *Handler) unmarshallUserData(data []byte) (*models.User, error) {
	var user models.User
	err := json.Unmarshal(data, &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (h *Handler) handleCentralSystemCommand(payload []byte) (*models.CentralSystemResponse, error) {
	var command models.CentralSystemCommand
	err := json.Unmarshal(payload, &command)
	if err != nil {
		return nil, fmt.Errorf("decoding central system command: %s", err)
	}
	if h.centralSystem != nil {
		response, err := h.centralSystem.SendCommand(&command)
		if err != nil {
			return nil, fmt.Errorf("sending command to central system: %s", err)
		}
		return response, nil
	} else {
		return models.NewCentralSystemResponse(models.Error, "central system is not connected"), nil
	}
}

func (h *Handler) HandleUserRequest(request *models.UserRequest) ([]byte, error) {
	var response *models.CentralSystemResponse
	var err error

	if h.centralSystem != nil {

		command := models.CentralSystemCommand{
			ChargePointId: request.ChargePointId,
			ConnectorId:   request.ConnectorId,
		}

		switch request.Command {
		case models.StartTransaction:
			command.FeatureName = "RemoteStartTransaction"
			command.Payload = request.Token
		case models.StopTransaction:
			command.FeatureName = "RemoteStopTransaction"
			command.Payload = fmt.Sprintf("%d", request.TransactionId)
		default:
			response = models.NewCentralSystemResponse(models.Error, fmt.Sprintf("unknown command %s", request.Command))
			return getByteData(response)
		}

		response, err = h.centralSystem.SendCommand(&command)
		if err != nil {
			h.logger.Error("sending command to central system", err)
			response = models.NewCentralSystemResponse(models.Error, "failed to send command")
			return getByteData(response)
		}

		response = models.NewCentralSystemResponse(models.Success, fmt.Sprintf("command %s was sent for %s connector %v", request.Command, request.ChargePointId, request.ConnectorId))
	} else {
		response = models.NewCentralSystemResponse(models.Error, "central system is not connected")
	}

	return getByteData(response)
}

func getByteData(data interface{}) ([]byte, error) {
	byteData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encoding data: %s", err)
	}
	return byteData, nil
}

func (h *Handler) getUserById(id string) (user *models.User) {
	var err error
	if h.database != nil {
		user, err = h.database.GetUserById(id)
		if err != nil {
			h.logger.Error("getting user", err)
		}
	}
	return user
}

func (h *Handler) generateKey(length int) string {
	b := make([]byte, 20)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	s := hex.EncodeToString(b)
	if len(s) > length {
		s = s[:length]
	}
	return s
}
