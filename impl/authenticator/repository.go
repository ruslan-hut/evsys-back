package authenticator

import "evsys-back/entity"

type Repository interface {
	GetUser(username string) (*entity.User, error)
	GetUserById(userId string) (*entity.User, error)
	UpdateLastSeen(user *entity.User) error
	AddUser(user *entity.User) error
	CheckUsername(username string) error
	CheckToken(token string) (*entity.User, error)
	AddInviteCode(invite *entity.Invite) error
	CheckInviteCode(code string) (bool, error)
	DeleteInviteCode(code string) error
	GetUserTags(userId string) ([]entity.UserTag, error)
	AddUserTag(userTag *entity.UserTag) error
	CheckUserTag(idTag string) error
	UpdateTagLastSeen(userTag *entity.UserTag) error
	GetUsers() ([]*entity.User, error)
}
