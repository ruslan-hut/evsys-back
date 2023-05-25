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

	GetChargePoints() (interface{}, error)

	GetTransaction(id int) (*models.Transaction, error)
	GetTransactionByTag(idTag string, timeStart time.Time) (*models.Transaction, error)
	GetTransactionState(id int) (*models.ChargeState, error)
	GetActiveTransactions(userId string) ([]*models.ChargeState, error)

	GetLastMeterValue(transactionId int) (*models.TransactionMeter, error)

	CheckInviteCode(code string) (bool, error)
	DeleteInviteCode(code string) error
}

type Data interface {
	DataType() string
}
