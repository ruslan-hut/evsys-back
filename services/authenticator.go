package services

import (
	"evsys-back/models"
)

type Auth interface {
	AuthenticateUser(username, password string) (*models.User, error)
	RegisterUser(user *models.User) error
	GenerateInvites(count int) ([]string, error)
	AuthenticateByToken(token string) (*models.User, error)
	GetUserTag(user *models.User) (string, error)
	GetUsers(role string) ([]*models.User, error)
}
