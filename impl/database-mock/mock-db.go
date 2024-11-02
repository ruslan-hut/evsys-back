package database_mock

import (
	"evsys-back/entity"
	"time"
)

type MockDB struct {
}

func NewMockDB() *MockDB {
	return &MockDB{}
}

func (db *MockDB) ReadLog(logName string) (interface{}, error) {
	return nil, nil
}

func (db *MockDB) ReadLogAfter(timeStart time.Time) ([]*entity.FeatureMessage, error) {
	return nil, nil
}

func (db *MockDB) GetConfig(name string) (interface{}, error) {
	return nil, nil
}

func (db *MockDB) GetUser(username string) (*entity.User, error) {
	return nil, nil
}

func (db *MockDB) GetUserInfo(level int, userId string) (*entity.UserInfo, error) {
	return nil, nil
}

func (db *MockDB) GetUsers() ([]*entity.User, error) {
	return nil, nil
}

func (db *MockDB) GetUserById(userId string) (*entity.User, error) {
	return nil, nil
}

func (db *MockDB) UpdateLastSeen(user *entity.User) error {
	return nil
}

func (db *MockDB) AddUser(user *entity.User) error {
	return nil
}

func (db *MockDB) CheckUsername(username string) error {
	return nil
}

func (db *MockDB) CheckToken(token string) (*entity.User, error) {
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

func (db *MockDB) GetUserTags(userId string) ([]entity.UserTag, error) {
	return nil, nil
}

func (db *MockDB) AddUserTag(userTag *entity.UserTag) error {
	return nil
}

func (db *MockDB) CheckUserTag(idTag string) error {
	return nil
}

func (db *MockDB) UpdateTagLastSeen(userTag *entity.UserTag) error {
	return nil
}

func (db *MockDB) GetLocations() ([]*entity.Location, error) {
	return nil, nil
}

func (db *MockDB) GetChargePoints(level int, searchTerm string) ([]*entity.ChargePoint, error) {
	return nil, nil
}

func (db *MockDB) GetChargePoint(level int, id string) (*entity.ChargePoint, error) {
	return nil, nil
}

func (db *MockDB) UpdateChargePoint(level int, chargePoint *entity.ChargePoint) error {
	return nil
}

func (db *MockDB) GetTransaction(id int) (*entity.Transaction, error) {
	return nil, nil
}

func (db *MockDB) GetTransactionByTag(idTag string, timeStart time.Time) (*entity.Transaction, error) {
	return nil, nil
}

func (db *MockDB) GetTransactionState(userId string, level int, id int) (*entity.ChargeState, error) {
	return nil, nil
}

func (db *MockDB) GetActiveTransactions(userId string) ([]*entity.ChargeState, error) {
	return nil, nil
}

func (db *MockDB) GetTransactions(userId string, period string) ([]*entity.Transaction, error) {
	return nil, nil
}

func (db *MockDB) GetTransactionsToBill(userId string) ([]*entity.Transaction, error) {
	return nil, nil
}

func (db *MockDB) UpdateTransaction(transaction *entity.Transaction) error {
	return nil
}

func (db *MockDB) GetLastMeterValue(transactionId int) (*entity.TransactionMeter, error) {
	return nil, nil
}

func (db *MockDB) GetMeterValues(transactionId int, from time.Time) ([]*entity.TransactionMeter, error) {
	return nil, nil
}

func (db *MockDB) AddInviteCode(invite *entity.Invite) error {
	return nil
}

func (db *MockDB) CheckInviteCode(code string) (bool, error) {
	return false, nil
}

func (db *MockDB) DeleteInviteCode(code string) error {
	return nil
}

func (db *MockDB) SavePaymentResult(paymentParameters *entity.PaymentParameters) error {
	return nil
}

func (db *MockDB) SavePaymentMethod(paymentMethod *entity.PaymentMethod) error {
	return nil
}

func (db *MockDB) UpdatePaymentMethod(paymentMethod *entity.PaymentMethod) error {
	return nil
}

func (db *MockDB) DeletePaymentMethod(paymentMethod *entity.PaymentMethod) error {
	return nil
}

func (db *MockDB) GetPaymentMethods(userId string) ([]*entity.PaymentMethod, error) {
	return nil, nil
}

func (db *MockDB) GetPaymentParameters(orderId string) (*entity.PaymentParameters, error) {
	return nil, nil
}

func (db *MockDB) GetLastOrder() (*entity.PaymentOrder, error) {
	return nil, nil
}

func (db *MockDB) SavePaymentOrder(order *entity.PaymentOrder) error {
	return nil
}

func (db *MockDB) GetPaymentOrder(id int) (*entity.PaymentOrder, error) {
	return nil, nil
}

func (db *MockDB) GetPaymentOrderByTransaction(transactionId int) (*entity.PaymentOrder, error) {
	return nil, nil
}

func (db *MockDB) TotalsByMonth(from, to time.Time, userGroup string) ([]interface{}, error) {
	return nil, nil
}
