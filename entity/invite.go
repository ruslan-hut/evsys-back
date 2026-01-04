package entity

type Invite struct {
	Code string `json:"code" bson:"code" validate:"required,min=6"`
}
