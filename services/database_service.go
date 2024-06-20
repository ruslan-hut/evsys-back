package services

import (
	"evsys-back/entity"
	"time"
)

type Database interface {
	WriteLogMessage(data Data) error
	ReadLog(logName string) (interface{}, error)
	ReadLogAfter(timeStart time.Time) ([]*FeatureMessage, error)
	GetConfig(name string) (interface{}, error)

	GetUser(username string) (*entity.User, error)
	GetUserInfo(level int, userId string) (*entity.UserInfo, error)
	GetUsers() ([]*entity.User, error)
	GetUserById(userId string) (*entity.User, error)
	UpdateLastSeen(user *entity.User) error
	AddUser(user *entity.User) error
	CheckUsername(username string) error
	CheckToken(token string) (*entity.User, error)

	GetUserTags(userId string) ([]entity.UserTag, error)
	AddUserTag(userTag *entity.UserTag) error
	CheckUserTag(idTag string) error
	UpdateTagLastSeen(userTag *entity.UserTag) error

	GetLocations() ([]*entity.Location, error)

	GetChargePoints(level int, searchTerm string) ([]*entity.ChargePoint, error)
	GetChargePoint(level int, id string) (*entity.ChargePoint, error)
	UpdateChargePoint(level int, chargePoint *entity.ChargePoint) error

	GetTransaction(id int) (*entity.Transaction, error)
	GetTransactionByTag(idTag string, timeStart time.Time) (*entity.Transaction, error)
	GetTransactionState(level int, id int) (*entity.ChargeState, error)
	GetActiveTransactions(userId string) ([]*entity.ChargeState, error)
	GetTransactions(userId string, period string) ([]*entity.Transaction, error)
	GetTransactionsToBill(userId string) ([]*entity.Transaction, error)
	UpdateTransaction(transaction *entity.Transaction) error

	GetLastMeterValue(transactionId int) (*entity.TransactionMeter, error)
	GetMeterValues(transactionId int, from time.Time) ([]*entity.TransactionMeter, error)

	AddInviteCode(invite *entity.Invite) error
	CheckInviteCode(code string) (bool, error)
	DeleteInviteCode(code string) error

	SavePaymentResult(paymentParameters *entity.PaymentParameters) error
	SavePaymentMethod(paymentMethod *entity.PaymentMethod) error
	UpdatePaymentMethod(paymentMethod *entity.PaymentMethod) error
	DeletePaymentMethod(paymentMethod *entity.PaymentMethod) error
	GetPaymentMethods(userId string) ([]*entity.PaymentMethod, error)
	GetPaymentParameters(orderId string) (*entity.PaymentParameters, error)

	GetLastOrder() (*entity.PaymentOrder, error)
	SavePaymentOrder(order *entity.PaymentOrder) error
	GetPaymentOrder(id int) (*entity.PaymentOrder, error)
	GetPaymentOrderByTransaction(transactionId int) (*entity.PaymentOrder, error)
}

type Data interface {
	DataType() string
}
