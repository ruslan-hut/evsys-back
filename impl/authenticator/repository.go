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
	CheckUsername(ctx context.Context, username string) error
	CheckToken(ctx context.Context, token string) (*entity.User, error)
	AddInviteCode(ctx context.Context, invite *entity.Invite) error
	CheckInviteCode(ctx context.Context, code string) (bool, error)
	DeleteInviteCode(ctx context.Context, code string) error
	GetUserTags(ctx context.Context, userId string) ([]entity.UserTag, error)
	AddUserTag(ctx context.Context, userTag *entity.UserTag) error
	CheckUserTag(ctx context.Context, idTag string) error
	UpdateTagLastSeen(ctx context.Context, userTag *entity.UserTag) error
	GetUsers(ctx context.Context) ([]*entity.User, error)
}
