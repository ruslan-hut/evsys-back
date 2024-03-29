package models

import "time"

type User struct {
	Username       string    `json:"username" bson:"username"`
	Password       string    `json:"password" bson:"password"`
	Name           string    `json:"name" bson:"name"`
	Role           string    `json:"role" bson:"role"`
	AccessLevel    int       `json:"access_level" bson:"access_level"`
	Email          string    `json:"email" bson:"email"`
	PaymentPlan    string    `json:"payment_plan" bson:"payment_plan"`
	Token          string    `json:"token" bson:"token"`
	UserId         string    `json:"user_id" bson:"user_id"`
	DateRegistered time.Time `json:"date_registered" bson:"date_registered"`
	LastSeen       time.Time `json:"last_seen" bson:"last_seen"`
}

type UserInfo struct {
	Username       string           `json:"username" bson:"username"`
	Name           string           `json:"name" bson:"name"`
	Role           string           `json:"role" bson:"role"`
	AccessLevel    int              `json:"access_level" bson:"access_level"`
	Email          string           `json:"email" bson:"email"`
	DateRegistered time.Time        `json:"date_registered" bson:"date_registered"`
	LastSeen       time.Time        `json:"last_seen" bson:"last_seen"`
	PaymentPlans   []*PaymentPlan   `json:"payment_plans" bson:"payment_plans"`
	UserTags       []*UserTag       `json:"user_tags" bson:"user_tags"`
	PaymentMethods []*PaymentMethod `json:"payment_methods" bson:"payment_methods"`
}
