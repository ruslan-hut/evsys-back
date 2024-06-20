package services

import "evsys-back/entity"

type Payments interface {
	Notify(data []byte) error
	SavePaymentMethod(user *entity.User, data []byte) error
	UpdatePaymentMethod(user *entity.User, data []byte) error
	DeletePaymentMethod(user *entity.User, data []byte) error
	SetOrder(user *entity.User, data []byte) (*entity.PaymentOrder, error)
}
