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
	ReadSysLog           CallType = "ReadSysLog"
	ReadBackLog          CallType = "ReadBackLog"
	AuthenticateUser     CallType = "AuthenticateUser"
	RegisterUser         CallType = "RegisterUser"
	GetChargePoints      CallType = "GetChargePoints"
	CentralSystemCommand CallType = "CentralSystemCommand"
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
		token := h.generateKey(32)
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

	if h.database == nil {
		response = models.NewCentralSystemResponse(models.Error, "database is not connected")
		return getByteData(response)
	}

	if request.UserId == "" {
		response = models.NewCentralSystemResponse(models.Error, "empty user id")
		return getByteData(response)
	}

	user, err := h.database.GetUserById(request.UserId)
	if err != nil {
		h.logger.Error("getting user from database", err)
	}
	if user == nil {
		username := fmt.Sprintf("user_%s", h.generateKey(5))
		user = &models.User{
			Username: username,
			Name:     request.Username,
			UserId:   request.UserId,
		}
		err = h.database.AddUser(user)
		if err != nil {
			h.logger.Error("adding user to database", err)
			response = models.NewCentralSystemResponse(models.Error, "failed to add user to database")
			return getByteData(response)
		}
	}
	if user.Name != request.Username {
		user.Name = request.Username
		err = h.database.UpdateUser(user)
		if err != nil {
			h.logger.Error("updating user in database", err)
		}
	}

	tags, err := h.database.GetUserTags(request.UserId)
	if err != nil {
		h.logger.Error("getting user tags from database", err)
	}
	if tags == nil {
		tags = make([]models.UserTag, 0)
	}
	if len(tags) == 0 {
		newTag := models.UserTag{
			UserId:    request.UserId,
			Username:  user.Username,
			IdTag:     h.generateKey(20),
			IsEnabled: true,
		}
		err = h.database.AddUserTag(&newTag)
		if err != nil {
			h.logger.Error("adding user tag to database", err)
			response = models.NewCentralSystemResponse(models.Error, "failed to add user tag to database")
			return getByteData(response)
		}
		tags = append(tags, newTag)
	}
	idTag := tags[0].IdTag

	if h.centralSystem != nil {

		command := models.CentralSystemCommand{
			ChargePointId: request.ChargePointId,
			ConnectorId:   request.ConnectorId,
		}

		switch request.Command {
		case models.StartTransaction:
			command.FeatureName = "RemoteStartTransaction"
			command.Payload = idTag
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
