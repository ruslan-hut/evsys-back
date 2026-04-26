package core

import (
	"context"
	"evsys-back/entity"
	"time"
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
	GetFilteredTransactions(ctx context.Context, filter *entity.TransactionFilter) ([]*entity.Transaction, error)
	GetTransactionState(ctx context.Context, userId string, level int, id int) (*entity.ChargeState, error)
	GetRecentUserChargePoints(ctx context.Context, userId string) ([]*entity.ChargePoint, error)

	GetPaymentMethods(ctx context.Context, userId string) ([]*entity.PaymentMethod, error)
	SavePaymentMethod(ctx context.Context, paymentMethod *entity.PaymentMethod) error
	UpdatePaymentMethod(ctx context.Context, paymentMethod *entity.PaymentMethod) error
	DeletePaymentMethod(ctx context.Context, paymentMethod *entity.PaymentMethod) error
	GetLastOrder(ctx context.Context) (*entity.PaymentOrder, error)
	SavePaymentOrder(ctx context.Context, order *entity.PaymentOrder) error
	GetPaymentOrderByTransaction(ctx context.Context, transactionId int) (*entity.PaymentOrder, error)

	// Direct payment methods
	GetTransaction(ctx context.Context, id int) (*entity.Transaction, error)
	UpdateTransactionPayment(ctx context.Context, transaction *entity.Transaction) error
	GetUserTag(ctx context.Context, idTag string) (*entity.UserTag, error)
	GetDefaultPaymentMethod(ctx context.Context, userId string) (*entity.PaymentMethod, error)
	GetPaymentMethodByIdentifier(ctx context.Context, identifier string) (*entity.PaymentMethod, error)
	UpdatePaymentMethodFailCount(ctx context.Context, identifier string, count int) error
	GetPaymentOrder(ctx context.Context, id int) (*entity.PaymentOrder, error)
	SavePaymentResult(ctx context.Context, paymentParameters *entity.PaymentParameters) error

	// Payment processing methods
	GetUnbilledTransactions(ctx context.Context) ([]*entity.Transaction, error)

	// Payment retry methods
	SavePaymentRetry(ctx context.Context, retry *entity.PaymentRetry) error
	GetPaymentRetry(ctx context.Context, transactionId int) (*entity.PaymentRetry, error)
	GetPendingRetries(ctx context.Context, now time.Time) ([]*entity.PaymentRetry, error)
	DeletePaymentRetry(ctx context.Context, transactionId int) error

	// Preauthorization methods
	SavePreauthorization(ctx context.Context, preauth *entity.Preauthorization) error
	GetPreauthorization(ctx context.Context, orderNumber string) (*entity.Preauthorization, error)
	GetPreauthorizationByTransaction(ctx context.Context, transactionId int) (*entity.Preauthorization, error)
	UpdatePreauthorization(ctx context.Context, preauth *entity.Preauthorization) error
	GetLastPreauthorizationOrder(ctx context.Context) (*entity.Preauthorization, error)

	// Mail subscriptions
	ListMailSubscriptions(ctx context.Context) ([]*entity.MailSubscription, error)
	ListMailSubscriptionsByPeriod(ctx context.Context, period string) ([]*entity.MailSubscription, error)
	GetMailSubscription(ctx context.Context, id string) (*entity.MailSubscription, error)
	SaveMailSubscription(ctx context.Context, sub *entity.MailSubscription) (*entity.MailSubscription, error)
	DeleteMailSubscription(ctx context.Context, id string) error
}
