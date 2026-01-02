package core

import (
	"context"
	"evsys-back/entity"
)

type Authenticator interface {
	AuthenticateByToken(ctx context.Context, token string) (*entity.User, error)
	AuthenticateUser(ctx context.Context, username, password string) (*entity.User, error)
	RegisterUser(ctx context.Context, user *entity.User) error
	GetUsers(ctx context.Context, user *entity.User) ([]*entity.User, error)
	GetUserTag(ctx context.Context, user *entity.User) (string, error)
	CommandAccess(user *entity.User, command string) error
	HasAccess(user *entity.User, subSystem string) error
	CreateUser(ctx context.Context, user *entity.User) error
	UpdateUser(ctx context.Context, username string, updates *entity.UserUpdate) (*entity.User, error)
	DeleteUser(ctx context.Context, username string) error
	// User tag management
	ListUserTags(ctx context.Context) ([]*entity.UserTag, error)
	GetUserTagByIdTag(ctx context.Context, idTag string) (*entity.UserTag, error)
	CreateUserTag(ctx context.Context, tag *entity.UserTagCreate) (*entity.UserTag, error)
	UpdateUserTag(ctx context.Context, idTag string, updates *entity.UserTagUpdate) (*entity.UserTag, error)
	DeleteUserTag(ctx context.Context, idTag string) error
}
