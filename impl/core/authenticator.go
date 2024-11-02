package core

import "evsys-back/entity"

type Authenticator interface {
	AuthenticateByToken(token string) (*entity.User, error)
	AuthenticateUser(username, password string) (*entity.User, error)
	RegisterUser(user *entity.User) error
	GetUsers(user *entity.User) ([]*entity.User, error)
	GetUserTag(user *entity.User) (string, error)
	CommandAccess(user *entity.User, command string) error
	HasAccess(user *entity.User, subSystem string) error
}
