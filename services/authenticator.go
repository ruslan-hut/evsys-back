package services

import "evsys-back/models"

type Auth interface {
	AuthenticateUser(username, password string) (*models.User, error)
	RegisterUser(user *models.User) error
	GetUser(token string) (*models.User, error)
	GetUserTag(userId string) (string, error)
}
