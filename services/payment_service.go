package services

type Payments interface {
	Notify(data []byte) error
	SavePaymentMethod(data []byte) error
}
