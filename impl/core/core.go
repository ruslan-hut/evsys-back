package core

import (
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"time"
)

const MaxAccessLevel int = 10

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
	return c.auth.AuthenticateByToken(token)
}

func (c *Core) AuthenticateUser(username, password string) (*entity.User, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	return c.auth.AuthenticateUser(username, password)
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
	return c.repo.GetChargePoints(accessLevel, search)
}

func (c *Core) GetChargePoint(accessLevel int, id string) (interface{}, error) {
	return c.repo.GetChargePoint(accessLevel, id)
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
	return c.repo.GetActiveTransactions(userId)
}

func (c *Core) GetTransactions(userId, period string) (interface{}, error) {
	return c.repo.GetTransactions(userId, period)
}

func (c *Core) GetTransaction(accessLevel, id int) (interface{}, error) {
	return c.repo.GetTransactionState(accessLevel, id)
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
	if user.UserId != pm.UserId && user.AccessLevel < MaxAccessLevel {
		return fmt.Errorf("access denied")
	}
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
