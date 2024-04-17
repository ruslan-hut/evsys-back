package internal

import (
	"encoding/json"
	"evsys-back/models"
	"evsys-back/services"
	"fmt"
	"net/http"
	"strconv"
)

type CallType string

const (
	ReadLog              CallType = "ReadLog"
	AuthenticateUser     CallType = "AuthenticateUser"
	RegisterUser         CallType = "RegisterUser"
	UserInfo             CallType = "UserInfo"
	UsersList            CallType = "UsersList"
	GetChargePoints      CallType = "GetChargePoints"
	ChargePointInfo      CallType = "ChargePointInfo"
	ChargePointUpdate    CallType = "ChargePointUpdate"
	CentralSystemCommand CallType = "CentralSystemCommand"
	ActiveTransactions   CallType = "ActiveTransactions"
	TransactionInfo      CallType = "TransactionInfo"
	TransactionList      CallType = "TransactionList"
	TransactionBill      CallType = "TransactionBill"
	GenerateInvites      CallType = "GenerateInvites"
	PaymentMethods       CallType = "PaymentMethods"
	GetConfig            CallType = "GetConfig"
	Locations            CallType = "Locations"

	MaxAccessLevel int = 10
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
	if h.auth == nil {
		h.logger.Warn("authenticator not initialized")
		return nil, http.StatusInternalServerError
	}

	userId := ""
	username := ""
	userRole := ""
	accessLevel := 0

	var data interface{}
	var err error
	status := http.StatusOK

	if ac.CallType != AuthenticateUser && ac.CallType != RegisterUser && ac.CallType != GetConfig {
		user, e := h.auth.AuthenticateByToken(ac.Token)
		if e != nil {
			h.logger.Error("token check failed", e)
			return nil, http.StatusUnauthorized
		}
		accessLevel = user.AccessLevel
		userId = user.UserId
		username = user.Username
		userRole = user.Role
	}

	switch ac.CallType {
	case ReadLog:
		data, err = h.database.ReadLog(string(ac.Payload))
		if err != nil {
			h.logger.Error("read log", err)
			status = http.StatusNoContent
		}
	case GetConfig:
		data, err = h.database.GetConfig(string(ac.Payload))
		if err != nil {
			h.logger.Error("get config", err)
			status = http.StatusNoContent
		}
	case AuthenticateUser:
		userData, err := h.unmarshallUserData(ac.Payload)
		if err != nil {
			h.logger.Error("decoding user", err)
			status = http.StatusUnsupportedMediaType
		} else {
			if userData.Username == "" {
				data, err = h.auth.AuthenticateByToken(userData.Password)
			} else {
				data, err = h.auth.AuthenticateUser(userData.Username, userData.Password)
			}
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
			if userData.AccessLevel > MaxAccessLevel {
				userData.AccessLevel = MaxAccessLevel
			}
			err = h.auth.RegisterUser(userData)
			if err != nil {
				h.logger.Error("user registration", err)
				status = http.StatusInternalServerError
			}
		}
	case UserInfo:
		requestedUserName := string(ac.Payload)
		if accessLevel < 10 || requestedUserName == "0000" { // user can get info about himself
			data, err = h.database.GetUserInfo(accessLevel, username)
			if err != nil {
				h.logger.Error("get user info", err)
				status = http.StatusNoContent
			}
		} else {
			data, err = h.database.GetUserInfo(accessLevel, requestedUserName)
			if err != nil {
				h.logger.Error("get user info", err)
				status = http.StatusNoContent
			}
		}
	case UsersList:
		data, err = h.auth.GetUsers(userRole)
		if err != nil {
			h.logger.Warn(fmt.Sprintf("get users: %s", err))
			status = http.StatusNoContent
			data = []models.User{}
		}
	case GenerateInvites:
		data, err = h.auth.GenerateInvites(5)
		if err != nil {
			h.logger.Error("generate invites", err)
			status = http.StatusInternalServerError
		}
	case GetChargePoints:
		search := string(ac.Payload)
		data, err = h.database.GetChargePoints(accessLevel, search)
		if err != nil {
			h.logger.Warn(fmt.Sprintf("no data by search: %s", search))
			status = http.StatusNoContent
		}
	case ChargePointInfo:
		id := string(ac.Payload)
		data, err = h.database.GetChargePoint(accessLevel, id)
		if err != nil {
			h.logger.Warn(fmt.Sprintf("not found charge point: %s", id))
			status = http.StatusNoContent
		}
	case ChargePointUpdate:
		chp, err := models.GetChargePointFromPayload(ac.Payload)
		if err != nil {
			h.logger.Error("decoding charge point", err)
			h.logger.Info(fmt.Sprintf("%s", ac.Payload))
			status = http.StatusUnsupportedMediaType
		} else {
			if chp.AccessLevel > MaxAccessLevel {
				chp.AccessLevel = MaxAccessLevel
			}
			err = h.database.UpdateChargePoint(accessLevel, chp)
			if err != nil {
				h.logger.Error("update charge point", err)
				status = http.StatusInternalServerError
			}
		}
		data, err = h.database.GetChargePoint(accessLevel, chp.Id)
		if err != nil {
			h.logger.Warn(fmt.Sprintf("not found charge point: %s", chp.Id))
			status = http.StatusNoContent
		}
	case PaymentMethods:
		data, err = h.database.GetPaymentMethods(userId)
		if err != nil {
			h.logger.Warn(fmt.Sprintf("no payment methods for %s", username))
			status = http.StatusNoContent
			data = []models.PaymentMethod{}
		}
	case ActiveTransactions:
		data, err = h.database.GetActiveTransactions(userId)
		if err != nil {
			h.logger.Debug(fmt.Sprintf("active transactions for %s: %s", username, err))
			status = http.StatusNoContent
		}
	case TransactionList:
		data, err = h.database.GetTransactions(userId, string(ac.Payload))
		if err != nil {
			h.logger.Warn(fmt.Sprintf("transactions data for %s: %s", username, err))
			status = http.StatusNoContent
		}
	case TransactionBill:
		//data, err = h.database.GetTransactionsToBill(userId)
		//if err != nil {
		//	h.logger.Error("get bill transactions", err)
		//	status = http.StatusInternalServerError
		//}
		//**************************************************
		// billing transactions is disabled for client app
		status = http.StatusNoContent
		data = []models.Transaction{}
		//**************************************************
	case TransactionInfo:
		id, err := strconv.Atoi(string(ac.Payload))
		if err != nil {
			h.logger.Error("decoding transaction id", err)
			status = http.StatusBadRequest
		} else {
			data, err = h.database.GetTransactionState(accessLevel, id)
			if err != nil {
				h.logger.Error("get transaction", err)
				status = http.StatusInternalServerError
			}
		}
	case Locations:
		if accessLevel < MaxAccessLevel {
			err = fmt.Errorf("access denied")
			status = http.StatusUnauthorized
		} else {
			data, err = h.database.GetLocations()
			if err != nil {
				h.logger.Error("get locations", err)
				status = http.StatusNoContent
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
	if h.centralSystem == nil {
		return nil, fmt.Errorf("central system is not connected")
	}
	response := models.NewCentralSystemResponse(command.ChargePointId, command.ConnectorId)
	result, err := h.centralSystem.SendCommand(&command)
	if err != nil {
		response.Status = models.Error
		response.Info = err.Error()
		return response, nil
	}
	response.Info = result
	return response, nil
}

// HandleUserRequest handler for requests made via websocket
func (h *Handler) HandleUserRequest(request *models.UserRequest) error {
	var err error

	if h.centralSystem == nil {
		return fmt.Errorf("central system is not connected")
	}

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
	case models.CheckStatus:
		return nil
	case models.ListenTransaction:
		return nil
	case models.StopListenTransaction:
		return nil
	case models.ListenChargePoints:
		return nil
	case models.ListenLog:
		return nil
	case models.PingConnection:
		return nil
	default:
		return fmt.Errorf("unknown command %s", request.Command)
	}

	_, err = h.centralSystem.SendCommand(&command)
	if err != nil {
		h.logger.Error("sending command to central system", err)
		return fmt.Errorf("sending command to central system: %s", err)
	}

	return nil
}
