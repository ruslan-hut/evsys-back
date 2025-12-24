package database_mock

import (
	"context"
	"evsys-back/entity"
	"time"
)

type MockDB struct {
}

func NewMockDB() *MockDB {
	return &MockDB{}
}

func (db *MockDB) ReadLog(_ context.Context, logName string) (interface{}, error) {
	return nil, nil
}

func (db *MockDB) ReadLogAfter(_ context.Context, timeStart time.Time) ([]*entity.FeatureMessage, error) {
	return nil, nil
}

func (db *MockDB) GetConfig(_ context.Context, name string) (interface{}, error) {
	return nil, nil
}

func (db *MockDB) GetUser(_ context.Context, username string) (*entity.User, error) {
	return nil, nil
}

func (db *MockDB) GetUserInfo(_ context.Context, level int, userId string) (*entity.UserInfo, error) {
	return nil, nil
}

func (db *MockDB) GetUsers(_ context.Context) ([]*entity.User, error) {
	return nil, nil
}

func (db *MockDB) GetUserById(_ context.Context, userId string) (*entity.User, error) {
	return nil, nil
}

func (db *MockDB) UpdateLastSeen(_ context.Context, user *entity.User) error {
	return nil
}

func (db *MockDB) AddUser(_ context.Context, user *entity.User) error {
	return nil
}

func (db *MockDB) CheckUsername(_ context.Context, username string) error {
	return nil
}

func (db *MockDB) CheckToken(_ context.Context, token string) (*entity.User, error) {
	if token == "12345678901234567890123456789000" {
		return &entity.User{
			Username:    "test",
			AccessLevel: 1,
			UserId:      "0000000000",
			Role:        "user",
			LastSeen:    time.Now(),
		}, nil
	}
	return nil, nil
}

func (db *MockDB) GetUserTags(_ context.Context, userId string) ([]entity.UserTag, error) {
	return nil, nil
}

func (db *MockDB) AddUserTag(_ context.Context, userTag *entity.UserTag) error {
	return nil
}

func (db *MockDB) CheckUserTag(_ context.Context, idTag string) error {
	return nil
}

func (db *MockDB) UpdateTagLastSeen(_ context.Context, userTag *entity.UserTag) error {
	return nil
}

func (db *MockDB) GetLocations(_ context.Context) ([]*entity.Location, error) {
	return nil, nil
}

func (db *MockDB) GetChargePoints(_ context.Context, level int, searchTerm string) ([]*entity.ChargePoint, error) {
	return nil, nil
}

func (db *MockDB) GetChargePoint(_ context.Context, level int, id string) (*entity.ChargePoint, error) {
	return nil, nil
}

func (db *MockDB) UpdateChargePoint(_ context.Context, level int, chargePoint *entity.ChargePoint) error {
	return nil
}

func (db *MockDB) GetTransaction(_ context.Context, id int) (*entity.Transaction, error) {
	return nil, nil
}

func (db *MockDB) GetTransactionByTag(_ context.Context, idTag string, timeStart time.Time) (*entity.Transaction, error) {
	return nil, nil
}

func (db *MockDB) GetTransactionState(_ context.Context, userId string, level int, id int) (*entity.ChargeState, error) {
	return nil, nil
}

func (db *MockDB) GetActiveTransactions(_ context.Context, userId string) ([]*entity.ChargeState, error) {
	return nil, nil
}

func (db *MockDB) GetTransactions(_ context.Context, userId string, period string) ([]*entity.Transaction, error) {
	return nil, nil
}

func (db *MockDB) GetTransactionsToBill(userId string) ([]*entity.Transaction, error) {
	return nil, nil
}

func (db *MockDB) UpdateTransaction(transaction *entity.Transaction) error {
	return nil
}

func (db *MockDB) GetLastMeterValue(_ context.Context, transactionId int) (*entity.TransactionMeter, error) {
	return nil, nil
}

func (db *MockDB) GetMeterValues(_ context.Context, transactionId int, from time.Time) ([]*entity.TransactionMeter, error) {
	return nil, nil
}

func (db *MockDB) GetRecentUserChargePoints(_ context.Context, userId string) ([]*entity.ChargePoint, error) {
	return nil, nil
}

func (db *MockDB) AddInviteCode(_ context.Context, invite *entity.Invite) error {
	return nil
}

func (db *MockDB) CheckInviteCode(_ context.Context, code string) (bool, error) {
	return false, nil
}

func (db *MockDB) DeleteInviteCode(_ context.Context, code string) error {
	return nil
}

func (db *MockDB) SavePaymentResult(paymentParameters *entity.PaymentParameters) error {
	return nil
}

func (db *MockDB) SavePaymentMethod(_ context.Context, paymentMethod *entity.PaymentMethod) error {
	return nil
}

func (db *MockDB) UpdatePaymentMethod(_ context.Context, paymentMethod *entity.PaymentMethod) error {
	return nil
}

func (db *MockDB) DeletePaymentMethod(_ context.Context, paymentMethod *entity.PaymentMethod) error {
	return nil
}

func (db *MockDB) GetPaymentMethods(_ context.Context, userId string) ([]*entity.PaymentMethod, error) {
	return nil, nil
}

func (db *MockDB) GetPaymentParameters(orderId string) (*entity.PaymentParameters, error) {
	return nil, nil
}

func (db *MockDB) GetLastOrder(_ context.Context) (*entity.PaymentOrder, error) {
	return nil, nil
}

func (db *MockDB) SavePaymentOrder(_ context.Context, order *entity.PaymentOrder) error {
	return nil
}

func (db *MockDB) GetPaymentOrder(_ context.Context, id int) (*entity.PaymentOrder, error) {
	return nil, nil
}

func (db *MockDB) GetPaymentOrderByTransaction(_ context.Context, transactionId int) (*entity.PaymentOrder, error) {
	return nil, nil
}

func (db *MockDB) TotalsByMonth(_ context.Context, from, to time.Time, userGroup string) ([]interface{}, error) {
	return nil, nil
}

func (db *MockDB) TotalsByUsers(_ context.Context, from, to time.Time, userGroup string) ([]interface{}, error) {
	return nil, nil
}

func (db *MockDB) TotalsByCharger(_ context.Context, from, to time.Time, userGroup string) ([]interface{}, error) {
	return nil, nil
}
