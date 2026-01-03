package websocket

import (
	"context"
	"evsys-back/entity"
	"time"
)

// SubscriptionType defines the type of events a client subscribes to
type SubscriptionType string

const (
	Broadcast        SubscriptionType = "broadcast"
	LogEvent         SubscriptionType = "log-event"
	ChargePointEvent SubscriptionType = "charge-point-event"
)

// Core provides authentication and request processing for WebSocket clients
type Core interface {
	AuthenticateByToken(ctx context.Context, token string) (*entity.User, error)
	UserTag(ctx context.Context, user *entity.User) (string, error)
	WsRequest(request *entity.UserRequest) error
}

// StatusReader provides transaction state management for WebSocket clients
type StatusReader interface {
	GetTransactionAfter(ctx context.Context, userId string, after time.Time) (*entity.Transaction, error)
	GetTransaction(ctx context.Context, transactionId int) (*entity.Transaction, error)
	GetLastMeterValues(ctx context.Context, transactionId int, from time.Time) ([]entity.TransactionMeter, error)

	SaveStatus(userId string, stage entity.Stage, transactionId int) (time.Time, error)
	GetStatus(userId string) (*entity.UserStatus, bool)
	ClearStatus(userId string)

	ReadLogAfter(ctx context.Context, timeStart time.Time) ([]*entity.FeatureMessage, error)
}
