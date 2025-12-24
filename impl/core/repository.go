package core

import (
	"context"
	"evsys-back/entity"
)

type Repository interface {
	GetConfig(ctx context.Context, name string) (interface{}, error)
	ReadLog(ctx context.Context, name string) (interface{}, error)

	GetUserInfo(ctx context.Context, accessLevel int, username string) (*entity.UserInfo, error)

	GetLocations(ctx context.Context) ([]*entity.Location, error)
	GetChargePoints(ctx context.Context, level int, searchTerm string) ([]*entity.ChargePoint, error)
	GetChargePoint(ctx context.Context, level int, id string) (*entity.ChargePoint, error)
	UpdateChargePoint(ctx context.Context, level int, chargePoint *entity.ChargePoint) error

	GetActiveTransactions(ctx context.Context, userId string) ([]*entity.ChargeState, error)
	GetTransactions(ctx context.Context, userId string, period string) ([]*entity.Transaction, error)
	GetTransactionState(ctx context.Context, userId string, level int, id int) (*entity.ChargeState, error)
	GetRecentUserChargePoints(ctx context.Context, userId string) ([]*entity.ChargePoint, error)

	GetPaymentMethods(ctx context.Context, userId string) ([]*entity.PaymentMethod, error)
	SavePaymentMethod(ctx context.Context, paymentMethod *entity.PaymentMethod) error
	UpdatePaymentMethod(ctx context.Context, paymentMethod *entity.PaymentMethod) error
	DeletePaymentMethod(ctx context.Context, paymentMethod *entity.PaymentMethod) error
	GetLastOrder(ctx context.Context) (*entity.PaymentOrder, error)
	SavePaymentOrder(ctx context.Context, order *entity.PaymentOrder) error
	GetPaymentOrderByTransaction(ctx context.Context, transactionId int) (*entity.PaymentOrder, error)
}
