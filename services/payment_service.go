package services

import "evsys-back/models"

type Payments interface {
	Notify(data []byte) error
	SavePaymentMethod(user *models.User, data []byte) error
	SetOrder(user *models.User, data []byte) (*models.PaymentOrder, error)
}
