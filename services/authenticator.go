package services

import (
	"evsys-back/entity"
)

type Auth interface {
	AuthenticateUser(username, password string) (*entity.User, error)
	RegisterUser(user *entity.User) error
	GenerateInvites(count int) ([]string, error)
	AuthenticateByToken(token string) (*entity.User, error)
	GetUserTag(user *entity.User) (string, error)
	GetUsers(role string) ([]*entity.User, error)
}
