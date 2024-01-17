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

//type LogType string

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
	//CentralSystemLog     LogType  = "sys"
	//BackendLog           LogType  = "back"
	//PaymentLog           LogType  = "pay"
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

	var user *models.User
	userId := ""
	var data interface{}
	var err error
	status := http.StatusOK

	if ac.CallType != AuthenticateUser && ac.CallType != RegisterUser {
		user, err = h.auth.AuthenticateByToken(ac.Token)
		if err != nil {
			h.logger.Error("token check failed", err)
			return nil, http.StatusUnauthorized
		}
		userId = user.UserId
	}

	switch ac.CallType {
	case ReadLog:
		data, err = h.database.ReadLog(string(ac.Payload))
		if err != nil {
			h.logger.Error("read log", err)
			status = http.StatusNoContent
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
		username := string(ac.Payload)
		if user.AccessLevel < 10 && username != user.Username {
			h.logger.Warn(fmt.Sprintf("user %s tries to get info about %s", user.Username, username))
			status = http.StatusUnauthorized
		} else {
			data, err = h.database.GetUserInfo(user.AccessLevel, username)
			if err != nil {
				h.logger.Error("get user info", err)
				status = http.StatusNoContent
			}
		}
	case UsersList:
		data, err = h.auth.GetUsers(user.Role)
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
		data, err = h.database.GetChargePoints(user.AccessLevel, search)
		if err != nil {
			h.logger.Warn(fmt.Sprintf("no data by search: %s", search))
			status = http.StatusNoContent
		}
	case ChargePointInfo:
		id := string(ac.Payload)
		data, err = h.database.GetChargePoint(user.AccessLevel, id)
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
			err = h.database.UpdateChargePoint(user.AccessLevel, chp)
			if err != nil {
				h.logger.Error("update charge point", err)
				status = http.StatusInternalServerError
			}
		}
		data, err = h.database.GetChargePoint(user.AccessLevel, chp.Id)
		if err != nil {
			h.logger.Warn(fmt.Sprintf("not found charge point: %s", chp.Id))
			status = http.StatusNoContent
		}
	case PaymentMethods:
		data, err = h.database.GetPaymentMethods(userId)
		if err != nil {
			h.logger.Warn(fmt.Sprintf("no payment methods for %s", user.Username))
			status = http.StatusNoContent
			data = []models.PaymentMethod{}
		}
	case ActiveTransactions:
		data, err = h.database.GetActiveTransactions(userId)
		if err != nil {
			h.logger.Debug(fmt.Sprintf("active transactions for %s: %s", user.Username, err))
			status = http.StatusNoContent
		}
	case TransactionList:
		data, err = h.database.GetTransactions(userId, string(ac.Payload))
		if err != nil {
			h.logger.Warn(fmt.Sprintf("transactions data for %s: %s", user.Username, err))
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
			data, err = h.database.GetTransactionState(user.AccessLevel, id)
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
