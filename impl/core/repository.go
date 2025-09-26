package core

import "evsys-back/entity"

type Repository interface {
	GetConfig(name string) (interface{}, error)
	ReadLog(name string) (interface{}, error)

	GetUserInfo(accessLevel int, username string) (*entity.UserInfo, error)

	GetLocations() ([]*entity.Location, error)
	GetChargePoints(level int, searchTerm string) ([]*entity.ChargePoint, error)
	GetChargePoint(level int, id string) (*entity.ChargePoint, error)
	UpdateChargePoint(level int, chargePoint *entity.ChargePoint) error

	GetActiveTransactions(userId string) ([]*entity.ChargeState, error)
	GetTransactions(userId string, period string) ([]*entity.Transaction, error)
	GetTransactionState(userId string, level int, id int) (*entity.ChargeState, error)
	GetRecentUserChargePoints(userId string) ([]*entity.ChargePoint, error)

	GetPaymentMethods(userId string) ([]*entity.PaymentMethod, error)
	SavePaymentMethod(paymentMethod *entity.PaymentMethod) error
	UpdatePaymentMethod(paymentMethod *entity.PaymentMethod) error
	DeletePaymentMethod(paymentMethod *entity.PaymentMethod) error
	GetLastOrder() (*entity.PaymentOrder, error)
	SavePaymentOrder(order *entity.PaymentOrder) error
	GetPaymentOrderByTransaction(transactionId int) (*entity.PaymentOrder, error)
}
