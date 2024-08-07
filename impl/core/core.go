package core

import (
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"math"
	"time"
)

const (
	MaxAccessLevel              int = 10
	NormalizedMeterValuesLength     = 60
)

type Repository interface {
	GetConfig(name string) (interface{}, error)
	ReadLog(name string) (interface{}, error)

	GetUserInfo(accessLevel int, username string) (*entity.UserInfo, error)

	GetLocations() ([]*entity.Location, error)
	GetChargePoints(level int, searchTerm string) ([]*entity.ChargePoint, error)
	GetChargePoint(level int, id string) (*entity.ChargePoint, error)
	UpdateChargePoint(level int, chargePoint *entity.ChargePoint) error

	GetActiveTransactions(userId string) ([]*entity.ChargeState, error)
	GetTransactions(userId string, period string) ([]*entity.Transaction, error)
	GetTransactionState(level int, id int) (*entity.ChargeState, error)

	GetPaymentMethods(userId string) ([]*entity.PaymentMethod, error)
	SavePaymentMethod(paymentMethod *entity.PaymentMethod) error
	UpdatePaymentMethod(paymentMethod *entity.PaymentMethod) error
	DeletePaymentMethod(paymentMethod *entity.PaymentMethod) error
	GetLastOrder() (*entity.PaymentOrder, error)
	SavePaymentOrder(order *entity.PaymentOrder) error
	GetPaymentOrderByTransaction(transactionId int) (*entity.PaymentOrder, error)
}

type Authenticator interface {
	AuthenticateByToken(token string) (*entity.User, error)
	AuthenticateUser(username, password string) (*entity.User, error)
	RegisterUser(user *entity.User) error
	GetUsers(role string) ([]*entity.User, error)
	GetUserTag(user *entity.User) (string, error)
}

type CentralSystem interface {
	SendCommand(command *entity.CentralSystemCommand) *entity.CentralSystemResponse
}

type Core struct {
	repo Repository
	auth Authenticator
	cs   CentralSystem
	log  *slog.Logger
}

func New(log *slog.Logger, repo Repository) *Core {
	return &Core{
		repo: repo,
		log:  log.With(sl.Module("impl.core")),
	}
}

func (c *Core) SetAuth(auth Authenticator) {
	c.auth = auth
}

func (c *Core) SetCentralSystem(cs CentralSystem) {
	c.cs = cs
}

func (c *Core) GetConfig(name string) (interface{}, error) {
	return c.repo.GetConfig(name)
}

func (c *Core) GetLog(name string) (interface{}, error) {
	return c.repo.ReadLog(name)
}

func (c *Core) AuthenticateByToken(token string) (*entity.User, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	user, err := c.auth.AuthenticateByToken(token)
	if err != nil {
		return nil, err
	}
	if user != nil {
		user.Password = ""
	}
	return user, nil
}

func (c *Core) AuthenticateUser(username, password string) (*entity.User, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	user, err := c.auth.AuthenticateUser(username, password)
	if err != nil {
		return nil, err
	}
	if user != nil {
		user.Password = ""
	}
	return user, nil
}

func (c *Core) AddUser(user *entity.User) (*entity.User, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	err := c.auth.RegisterUser(user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (c *Core) GetUser(author *entity.User, username string) (*entity.UserInfo, error) {
	if author.AccessLevel < 10 || username == "0000" { // user can get info about himself
		return c.repo.GetUserInfo(author.AccessLevel, author.Username)
	}
	return c.repo.GetUserInfo(author.AccessLevel, username)
}

func (c *Core) GetUsers(_ int, role string) ([]*entity.User, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	return c.auth.GetUsers(role)
}

func (c *Core) UserTag(user *entity.User) (string, error) {
	if c.auth == nil {
		return "", fmt.Errorf("authenticator not set")
	}
	if user == nil {
		return "", fmt.Errorf("user is nil")
	}
	return c.auth.GetUserTag(user)
}

func (c *Core) GetLocations(accessLevel int) (interface{}, error) {
	if accessLevel < MaxAccessLevel {
		return nil, fmt.Errorf("access denied")
	}
	return c.repo.GetLocations()
}

func (c *Core) GetChargePoints(accessLevel int, search string) (interface{}, error) {
	list, err := c.repo.GetChargePoints(accessLevel, search)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return nil, nil
	}
	// disable connectors if charge point is not operational
	for _, cp := range list {
		cp.CheckConnectorsStatus()
		if accessLevel < MaxAccessLevel {
			cp.HideSensitiveData()
		}
	}
	return list, nil
}

func (c *Core) GetChargePoint(accessLevel int, id string) (interface{}, error) {
	cp, err := c.repo.GetChargePoint(accessLevel, id)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, nil
	}
	// disable connectors if charge point is not operational
	cp.CheckConnectorsStatus()
	if accessLevel < MaxAccessLevel {
		cp.HideSensitiveData()
	}
	return cp, nil
}

func (c *Core) SaveChargePoint(accessLevel int, chargePoint *entity.ChargePoint) error {
	if chargePoint == nil {
		return fmt.Errorf("charge point is nil")
	}
	if chargePoint.AccessLevel > MaxAccessLevel {
		chargePoint.AccessLevel = MaxAccessLevel
	}
	if accessLevel < chargePoint.AccessLevel {
		return fmt.Errorf("access denied")
	}
	return c.repo.UpdateChargePoint(accessLevel, chargePoint)
}

func (c *Core) SendCommand(command *entity.CentralSystemCommand) (interface{}, error) {
	if c.cs == nil {
		return nil, fmt.Errorf("central system not set")
	}
	if command == nil {
		return nil, fmt.Errorf("command is nil")
	}
	return c.cs.SendCommand(command), nil
}

func (c *Core) GetActiveTransactions(userId string) (interface{}, error) {
	transactions, err := c.repo.GetActiveTransactions(userId)
	if err != nil {
		return nil, err
	}
	if transactions == nil {
		empty := make([]*entity.ChargeState, 0)
		return empty, nil
	}
	for _, t := range transactions {
		t.MeterValues = NormalizeMeterValues(t.MeterValues, NormalizedMeterValuesLength)
	}
	return transactions, nil
}

func (c *Core) GetTransactions(userId, period string) (interface{}, error) {
	transactions, err := c.repo.GetTransactions(userId, period)
	if err != nil {
		return nil, err
	}
	if transactions == nil {
		empty := make([]*entity.Transaction, 0)
		return empty, nil
	}
	for _, t := range transactions {
		t.MeterValues = NormalizeMeterValues(t.MeterValues, NormalizedMeterValuesLength)
	}
	return transactions, nil
}

func (c *Core) GetTransaction(accessLevel, id int) (interface{}, error) {
	state, err := c.repo.GetTransactionState(accessLevel, id)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	state.MeterValues = NormalizeMeterValues(state.MeterValues, NormalizedMeterValuesLength)
	return state, nil
}

func (c *Core) GetPaymentMethods(userId string) (interface{}, error) {
	return c.repo.GetPaymentMethods(userId)
}

func (c *Core) SavePaymentMethod(user *entity.User, pm *entity.PaymentMethod) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	if pm == nil {
		return fmt.Errorf("payment method is nil")
	}
	pm.UserId = user.UserId
	pm.UserName = user.Username
	return c.repo.SavePaymentMethod(pm)
}

func (c *Core) UpdatePaymentMethod(user *entity.User, pm *entity.PaymentMethod) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	if pm == nil {
		return fmt.Errorf("payment method is nil")
	}
	pm.UserId = user.UserId
	pm.UserName = user.Username
	return c.repo.UpdatePaymentMethod(pm)
}

func (c *Core) DeletePaymentMethod(user *entity.User, pm *entity.PaymentMethod) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	if pm == nil {
		return fmt.Errorf("payment method is nil")
	}
	// delete only methods, belonging to user that requested deletion
	pm.UserId = user.UserId
	return c.repo.DeletePaymentMethod(pm)
}

func (c *Core) SetOrder(user *entity.User, order *entity.PaymentOrder) (*entity.PaymentOrder, error) {
	if user == nil {
		return nil, fmt.Errorf("user is nil")
	}
	if order == nil {
		return nil, fmt.Errorf("order is nil")
	}
	if order.Order == 0 {

		// if payment process was interrupted, find unclosed order and close it
		if order.TransactionId > 0 {
			continueOrder, _ := c.repo.GetPaymentOrderByTransaction(order.TransactionId)
			if continueOrder != nil {
				continueOrder.IsCompleted = true
				continueOrder.Result = "closed without response"
				continueOrder.TimeClosed = time.Now()
				_ = c.repo.SavePaymentOrder(continueOrder)
			}
		}

		lastOrder, _ := c.repo.GetLastOrder()
		if lastOrder != nil {
			order.Order = lastOrder.Order + 1
		} else {
			order.Order = 1200
		}
		order.TimeOpened = time.Now()
	}

	order.UserId = user.UserId
	order.UserName = user.Username

	err := c.repo.SavePaymentOrder(order)
	if err != nil {
		return nil, err
	}
	return order, nil
}

// WsRequest handler for requests made via websocket
func (c *Core) WsRequest(request *entity.UserRequest) error {
	if c.cs == nil {
		return fmt.Errorf("central system is not connected")
	}

	command := entity.CentralSystemCommand{
		ChargePointId: request.ChargePointId,
		ConnectorId:   request.ConnectorId,
	}

	switch request.Command {
	case entity.StartTransaction:
		command.FeatureName = "RemoteStartTransaction"
		command.Payload = request.Token
	case entity.StopTransaction:
		command.FeatureName = "RemoteStopTransaction"
		command.Payload = fmt.Sprintf("%d", request.TransactionId)
	case entity.CheckStatus:
		return nil
	case entity.ListenTransaction:
		return nil
	case entity.StopListenTransaction:
		return nil
	case entity.ListenChargePoints:
		return nil
	case entity.ListenLog:
		return nil
	case entity.PingConnection:
		return nil
	default:
		return fmt.Errorf("unknown command %s", request.Command)
	}

	response := c.cs.SendCommand(&command)
	if response.IsError() {
		return fmt.Errorf("sending command to central system: %s", response.Info)
	}

	return nil
}

func NormalizeMeterValues(meterValues []*entity.TransactionMeter, newLength int) []*entity.TransactionMeter {
	if newLength == 0 || meterValues == nil || len(meterValues) <= newLength {
		return meterValues
	}
	originalLength := len(meterValues)
	scaleFactor := float64(originalLength) / float64(newLength)
	normalized := make([]*entity.TransactionMeter, newLength)
	for i := 0; i < newLength; i++ {
		originalIndex := float64(i) * scaleFactor
		lowerIndex := int(math.Floor(originalIndex))
		upperIndex := int(math.Ceil(originalIndex))
		if upperIndex >= originalLength {
			upperIndex = originalLength - 1
		}
		normalized[i] = meterValues[lowerIndex]
		if lowerIndex != upperIndex {
			v1 := float64(meterValues[lowerIndex].ConsumedEnergy)
			v2 := float64(meterValues[upperIndex].ConsumedEnergy)
			normalized[i].ConsumedEnergy = int(interpolate(originalIndex, float64(lowerIndex), float64(upperIndex), v1, v2))
			p1 := float64(meterValues[lowerIndex].PowerRate)
			p2 := float64(meterValues[upperIndex].PowerRate)
			normalized[i].PowerRate = int(interpolate(originalIndex, float64(lowerIndex), float64(upperIndex), p1, p2))
		}
	}
	return normalized
}

// Function to perform linear interpolation
func interpolate(x, x0, x1, y0, y1 float64) float64 {
	return y0 + (y1-y0)*(x-x0)/(x1-x0)
}
