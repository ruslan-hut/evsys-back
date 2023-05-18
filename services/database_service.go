package services

import (
	"evsys-back/models"
)

type Database interface {
	WriteLogMessage(data Data) error
	ReadSystemLog() (interface{}, error)
	ReadBackLog() (interface{}, error)

	GetUser(username string) (*models.User, error)
	GetUserById(userId string) (*models.User, error)
	UpdateUser(user *models.User) error
	AddUser(user *models.User) error
	CheckToken(token string) (*models.User, error)

	GetUserTags(userId string) ([]models.UserTag, error)
	AddUserTag(userTag *models.UserTag) error

	GetChargePoints() (interface{}, error)

	GetTransaction(id int) (*models.ChargeState, error)
	GetActiveTransactions(userId string) ([]models.ChargeState, error)
}

type Data interface {
	DataType() string
}
