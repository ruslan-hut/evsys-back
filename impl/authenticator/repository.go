package authenticator

import (
	"context"
	"evsys-back/entity"
)

type Repository interface {
	GetUser(ctx context.Context, username string) (*entity.User, error)
	GetUserById(ctx context.Context, userId string) (*entity.User, error)
	UpdateLastSeen(ctx context.Context, user *entity.User) error
	AddUser(ctx context.Context, user *entity.User) error
	UpdateUser(ctx context.Context, user *entity.User) error
	DeleteUser(ctx context.Context, username string) error
	CheckUsername(ctx context.Context, username string) error
	CheckToken(ctx context.Context, token string) (*entity.User, error)
	AddInviteCode(ctx context.Context, invite *entity.Invite) error
	CheckInviteCode(ctx context.Context, code string) (bool, error)
	DeleteInviteCode(ctx context.Context, code string) error
	GetUserTags(ctx context.Context, userId string) ([]entity.UserTag, error)
	GetAllUserTags(ctx context.Context) ([]*entity.UserTag, error)
	GetUserTagByIdTag(ctx context.Context, idTag string) (*entity.UserTag, error)
	AddUserTag(ctx context.Context, userTag *entity.UserTag) error
	UpdateUserTag(ctx context.Context, userTag *entity.UserTag) error
	DeleteUserTag(ctx context.Context, idTag string) error
	CheckUserTag(ctx context.Context, idTag string) error
	UpdateTagLastSeen(ctx context.Context, userTag *entity.UserTag) error
	GetUsers(ctx context.Context) ([]*entity.User, error)
}
