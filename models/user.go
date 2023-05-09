package models

type User struct {
	Username string `json:"username" bson:"username"`
	Password string `json:"password" bson:"password"`
	Name     string `json:"name" bson:"name"`
	Role     string `json:"role" bson:"role"`
	Token    string `json:"token" bson:"token"`
	UserId   string `json:"user_id" bson:"user_id"`
}
