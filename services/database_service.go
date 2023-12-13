package services

import (
	"evsys-back/models"
	"time"
)

type Database interface {
	WriteLogMessage(data Data) error
	ReadLog(logName string) (interface{}, error)
	ReadLogAfter(timeStart time.Time) ([]*FeatureMessage, error)

	GetUser(username string) (*models.User, error)
	GetUsers() ([]*models.User, error)
	GetUserById(userId string) (*models.User, error)
	UpdateLastSeen(user *models.User) error
	AddUser(user *models.User) error
	CheckToken(token string) (*models.User, error)

	GetUserTags(userId string) ([]models.UserTag, error)
	AddUserTag(userTag *models.UserTag) error
	UpdateTagLastSeen(userTag *models.UserTag) error

	GetChargePoints(searchTerm string) ([]*models.ChargePoint, error)
	GetChargePoint(id string) (*models.ChargePoint, error)

	GetTransaction(id int) (*models.Transaction, error)
	GetTransactionByTag(idTag string, timeStart time.Time) (*models.Transaction, error)
	GetTransactionState(id int) (*models.ChargeState, error)
	GetActiveTransactions(userId string) ([]*models.ChargeState, error)
	GetTransactions(userId string, period string) ([]*models.Transaction, error)
	GetTransactionsToBill(userId string) ([]*models.Transaction, error)
	UpdateTransaction(transaction *models.Transaction) error

	GetLastMeterValue(transactionId int) (*models.TransactionMeter, error)

	AddInviteCode(invite *models.Invite) error
	CheckInviteCode(code string) (bool, error)
	DeleteInviteCode(code string) error

	SavePaymentResult(paymentParameters *models.PaymentParameters) error
	SavePaymentMethod(paymentMethod *models.PaymentMethod) error
	UpdatePaymentMethod(paymentMethod *models.PaymentMethod) error
	DeletePaymentMethod(paymentMethod *models.PaymentMethod) error
	GetPaymentMethods(userId string) ([]*models.PaymentMethod, error)
	GetPaymentParameters(orderId string) (*models.PaymentParameters, error)

	GetLastOrder() (*models.PaymentOrder, error)
	SavePaymentOrder(order *models.PaymentOrder) error
	GetPaymentOrder(id int) (*models.PaymentOrder, error)
	GetPaymentOrderByTransaction(transactionId int) (*models.PaymentOrder, error)
}

type Data interface {
	DataType() string
}
