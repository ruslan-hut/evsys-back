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
	UserId        string
	Time          time.Time
	Stage         Stage
	TransactionId int
}
