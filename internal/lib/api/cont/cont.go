package cont

import (
	"context"
	"evsys-back/models"
)

type ctxKey string

const UserDataKey ctxKey = "userData"

func PutUser(c context.Context, user *models.User) context.Context {
	return context.WithValue(c, UserDataKey, *user)
}

func GetUser(c context.Context) *models.User {
	user, ok := c.Value(UserDataKey).(models.User)
	if !ok {
		return &models.User{}
	}
	return &user
}
