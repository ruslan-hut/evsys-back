package services

import (
	"evsys-back/models"
)

type Database interface {
	WriteLogMessage(data Data) error
	ReadSystemLog() (interface{}, error)
	ReadBackLog() (interface{}, error)

	GetUser(username string) (*models.User, error)
	UpdateUser(user *models.User) error
	AddUser(user *models.User) error
	CheckToken(token string) error

	GetChargePoints() (interface{}, error)
}

type Data interface {
	DataType() string
}
