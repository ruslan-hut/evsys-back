package services

import "evsys-back/models"

type Auth interface {
	AuthenticateUser(username, password string) (*models.User, error)
	RegisterUser(user *models.User) error
	GenerateInvites(count int) ([]string, error)
	// GetUser returns user data by token, token is checked in database and firebase
	GetUser(token string) (*models.User, error)
	GetUserTag(userId string) (string, error)
}
