package services

import (
	"evsys-back/models"
	"time"
)

type Database interface {
	WriteLogMessage(data Data) error
	ReadSystemLog() (interface{}, error)
	ReadBackLog() (interface{}, error)
	ReadLogAfter(timeStart time.Time) ([]*FeatureMessage, error)

	GetUser(username string) (*models.User, error)
	GetUserById(userId string) (*models.User, error)
	UpdateUser(user *models.User) error
	AddUser(user *models.User) error
	CheckToken(token string) (*models.User, error)

	GetUserTags(userId string) ([]models.UserTag, error)
	AddUserTag(userTag *models.UserTag) error

	GetChargePoints(searchTerm string) (interface{}, error)

	GetTransaction(id int) (*models.Transaction, error)
	GetTransactionByTag(idTag string, timeStart time.Time) (*models.Transaction, error)
	GetTransactionState(id int) (*models.ChargeState, error)
	GetActiveTransactions(userId string) ([]*models.ChargeState, error)
	GetTransactions(userId string, limit int, offset int) ([]*models.Transaction, error)
	GetTransactionsToBill(userId string) ([]*models.Transaction, error)

	GetLastMeterValue(transactionId int) (*models.TransactionMeter, error)

	AddInviteCode(invite *models.Invite) error
	CheckInviteCode(code string) (bool, error)
	DeleteInviteCode(code string) error

	SavePaymentResult(paymentParameters *models.PaymentParameters) error
	SavePaymentMethod(paymentMethod *models.PaymentMethod) error
	GetPaymentMethods(userId string) ([]*models.PaymentMethod, error)
	GetPaymentParameters(orderId string) (*models.PaymentParameters, error)
}

type Data interface {
	DataType() string
}
