package entity

import (
	"time"
)

type Stage string

const (
	StageStart  Stage = "Start"
	StageStop   Stage = "Stop"
	StageListen Stage = "Listen"
)

type UserStatus struct {
	UserId        string `validate:"required"`
	Time          time.Time
	Stage         Stage `validate:"omitempty"`
	TransactionId int   `validate:"min=0"`
}
