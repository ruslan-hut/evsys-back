package database_mock

import (
	"context"
	"evsys-back/entity"
	"fmt"
	"sync"
	"time"
)

// MockDB provides an in-memory database implementation for testing
type MockDB struct {
	users             map[string]*entity.User             // key: username
	usersById         map[string]*entity.User             // key: userId
	tokens            map[string]*entity.User             // key: token
	userTags          map[string][]entity.UserTag         // key: userId
	allTags           map[string]bool                     // key: idTag (for uniqueness check)
	invites           map[string]*entity.Invite           // key: code
	transactions      map[int]*entity.Transaction         // key: transactionId
	chargeStates      map[int]*entity.ChargeState         // key: transactionId
	meterValues       map[int][]entity.TransactionMeter   // key: transactionId
	paymentMethods    map[string][]*entity.PaymentMethod  // key: userId
	paymentOrders     map[int]*entity.PaymentOrder        // key: orderId
	ordersByTx        map[int]*entity.PaymentOrder        // key: transactionId
	preauthorizations map[string]*entity.Preauthorization // key: orderNumber
	preauthByTx       map[int]*entity.Preauthorization    // key: transactionId
	lastOrderId       int
	mux               sync.RWMutex
}

// NewMockDB creates a new MockDB with initialized maps
func NewMockDB() *MockDB {
	return &MockDB{
		users:             make(map[string]*entity.User),
		usersById:         make(map[string]*entity.User),
		tokens:            make(map[string]*entity.User),
		userTags:          make(map[string][]entity.UserTag),
		allTags:           make(map[string]bool),
		invites:           make(map[string]*entity.Invite),
		transactions:      make(map[int]*entity.Transaction),
		chargeStates:      make(map[int]*entity.ChargeState),
		meterValues:       make(map[int][]entity.TransactionMeter),
		paymentMethods:    make(map[string][]*entity.PaymentMethod),
		paymentOrders:     make(map[int]*entity.PaymentOrder),
		ordersByTx:        make(map[int]*entity.PaymentOrder),
		preauthorizations: make(map[string]*entity.Preauthorization),
		preauthByTx:       make(map[int]*entity.Preauthorization),
		lastOrderId:       0,
	}
}

// Reset clears all data from the mock database
func (db *MockDB) Reset() {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.users = make(map[string]*entity.User)
	db.usersById = make(map[string]*entity.User)
	db.tokens = make(map[string]*entity.User)
	db.userTags = make(map[string][]entity.UserTag)
	db.allTags = make(map[string]bool)
	db.invites = make(map[string]*entity.Invite)
	db.transactions = make(map[int]*entity.Transaction)
	db.chargeStates = make(map[int]*entity.ChargeState)
	db.meterValues = make(map[int][]entity.TransactionMeter)
	db.paymentMethods = make(map[string][]*entity.PaymentMethod)
	db.paymentOrders = make(map[int]*entity.PaymentOrder)
	db.ordersByTx = make(map[int]*entity.PaymentOrder)
	db.preauthorizations = make(map[string]*entity.Preauthorization)
	db.preauthByTx = make(map[int]*entity.Preauthorization)
	db.lastOrderId = 0
}

// SeedUser adds a test user to the mock database
func (db *MockDB) SeedUser(user *entity.User) {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.users[user.Username] = user
	if user.UserId != "" {
		db.usersById[user.UserId] = user
	}
	if user.Token != "" {
		db.tokens[user.Token] = user
	}
}

// SeedInvite adds a test invite code to the mock database
func (db *MockDB) SeedInvite(code string) {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.invites[code] = &entity.Invite{Code: code}
}

// SeedTransaction adds a test transaction to the mock database
func (db *MockDB) SeedTransaction(tx *entity.Transaction) {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.transactions[tx.TransactionId] = tx
}

// SeedChargeState adds a test charge state to the mock database
func (db *MockDB) SeedChargeState(state *entity.ChargeState) {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.chargeStates[state.TransactionId] = state
}

// SeedPaymentOrder adds a test payment order to the mock database
func (db *MockDB) SeedPaymentOrder(order *entity.PaymentOrder) {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.paymentOrders[order.Order] = order
	if order.TransactionId > 0 {
		db.ordersByTx[order.TransactionId] = order
	}
	if order.Order > db.lastOrderId {
		db.lastOrderId = order.Order
	}
}

// --- User Methods ---

func (db *MockDB) GetUser(_ context.Context, username string) (*entity.User, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	user, ok := db.users[username]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

func (db *MockDB) GetUserById(_ context.Context, userId string) (*entity.User, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	user, ok := db.usersById[userId]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (db *MockDB) GetUsers(_ context.Context) ([]*entity.User, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	users := make([]*entity.User, 0, len(db.users))
	for _, user := range db.users {
		users = append(users, user)
	}
	return users, nil
}

func (db *MockDB) GetUserInfo(_ context.Context, level int, userId string) (*entity.UserInfo, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	user, ok := db.usersById[userId]
	if !ok {
		user, ok = db.users[userId]
	}
	if !ok {
		return nil, nil
	}
	return &entity.UserInfo{
		Username: user.Username,
		Name:     user.Name,
		Role:     user.Role,
	}, nil
}

func (db *MockDB) AddUser(_ context.Context, user *entity.User) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.users[user.Username] = user
	if user.UserId != "" {
		db.usersById[user.UserId] = user
	}
	if user.Token != "" {
		db.tokens[user.Token] = user
	}
	return nil
}

func (db *MockDB) UpdateUser(_ context.Context, user *entity.User) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	existing, ok := db.users[user.Username]
	if !ok {
		return fmt.Errorf("user not found")
	}
	// Update existing user fields
	existing.Name = user.Name
	existing.Email = user.Email
	existing.Role = user.Role
	existing.AccessLevel = user.AccessLevel
	existing.PaymentPlan = user.PaymentPlan
	existing.Password = user.Password
	return nil
}

func (db *MockDB) DeleteUser(_ context.Context, username string) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	user, ok := db.users[username]
	if !ok {
		return fmt.Errorf("user not found")
	}
	// Remove from all maps
	delete(db.users, username)
	if user.UserId != "" {
		delete(db.usersById, user.UserId)
	}
	if user.Token != "" {
		delete(db.tokens, user.Token)
	}
	return nil
}

func (db *MockDB) UpdateLastSeen(_ context.Context, user *entity.User) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	if existing, ok := db.users[user.Username]; ok {
		existing.LastSeen = user.LastSeen
		existing.Token = user.Token
	}
	return nil
}

func (db *MockDB) CheckUsername(_ context.Context, username string) error {
	db.mux.RLock()
	defer db.mux.RUnlock()
	if _, ok := db.users[username]; ok {
		return fmt.Errorf("username exists")
	}
	return nil
}

func (db *MockDB) CheckToken(_ context.Context, token string) (*entity.User, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	user, ok := db.tokens[token]
	if !ok {
		return nil, nil
	}
	return user, nil
}

// --- User Tags ---

func (db *MockDB) GetUserTags(_ context.Context, userId string) ([]entity.UserTag, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	tags, ok := db.userTags[userId]
	if !ok {
		return nil, nil
	}
	return tags, nil
}

func (db *MockDB) AddUserTag(_ context.Context, userTag *entity.UserTag) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.userTags[userTag.UserId] = append(db.userTags[userTag.UserId], *userTag)
	db.allTags[userTag.IdTag] = true
	return nil
}

func (db *MockDB) CheckUserTag(_ context.Context, idTag string) error {
	db.mux.RLock()
	defer db.mux.RUnlock()
	if _, ok := db.allTags[idTag]; ok {
		return fmt.Errorf("tag exists")
	}
	return nil
}

func (db *MockDB) UpdateTagLastSeen(_ context.Context, userTag *entity.UserTag) error {
	return nil
}

func (db *MockDB) GetAllUserTags(_ context.Context) ([]*entity.UserTag, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	result := make([]*entity.UserTag, 0)
	for _, tags := range db.userTags {
		for i := range tags {
			result = append(result, &tags[i])
		}
	}
	return result, nil
}

func (db *MockDB) GetUserTagByIdTag(_ context.Context, idTag string) (*entity.UserTag, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	for _, tags := range db.userTags {
		for i := range tags {
			if tags[i].IdTag == idTag {
				return &tags[i], nil
			}
		}
	}
	return nil, fmt.Errorf("tag not found")
}

func (db *MockDB) UpdateUserTag(_ context.Context, userTag *entity.UserTag) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	for userId, tags := range db.userTags {
		for i := range tags {
			if tags[i].IdTag == userTag.IdTag {
				// If user changed, move tag to new user
				if tags[i].UserId != userTag.UserId {
					// Remove from old user
					db.userTags[userId] = append(tags[:i], tags[i+1:]...)
					// Add to new user
					db.userTags[userTag.UserId] = append(db.userTags[userTag.UserId], *userTag)
				} else {
					// Update in place
					tags[i] = *userTag
				}
				return nil
			}
		}
	}
	return fmt.Errorf("tag not found")
}

func (db *MockDB) DeleteUserTag(_ context.Context, idTag string) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	for userId, tags := range db.userTags {
		for i := range tags {
			if tags[i].IdTag == idTag {
				db.userTags[userId] = append(tags[:i], tags[i+1:]...)
				delete(db.allTags, idTag)
				return nil
			}
		}
	}
	return fmt.Errorf("tag not found")
}

// --- Invite Codes ---

func (db *MockDB) AddInviteCode(_ context.Context, invite *entity.Invite) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.invites[invite.Code] = invite
	return nil
}

func (db *MockDB) CheckInviteCode(_ context.Context, code string) (bool, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	_, ok := db.invites[code]
	return ok, nil
}

func (db *MockDB) DeleteInviteCode(_ context.Context, code string) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	delete(db.invites, code)
	return nil
}

// --- Transactions ---

func (db *MockDB) GetTransaction(_ context.Context, id int) (*entity.Transaction, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	tx, ok := db.transactions[id]
	if !ok {
		return nil, nil
	}
	return tx, nil
}

func (db *MockDB) GetTransactionByTag(_ context.Context, idTag string, timeStart time.Time) (*entity.Transaction, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	for _, tx := range db.transactions {
		if tx.IdTag == idTag && tx.TimeStart.After(timeStart) {
			return tx, nil
		}
	}
	return nil, nil
}

func (db *MockDB) GetTransactionState(_ context.Context, userId string, level int, id int) (*entity.ChargeState, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	state, ok := db.chargeStates[id]
	if !ok {
		return nil, nil
	}
	return state, nil
}

func (db *MockDB) GetActiveTransactions(_ context.Context, userId string) ([]*entity.ChargeState, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()

	// Get user's tags
	tags, ok := db.userTags[userId]
	if !ok || len(tags) == 0 {
		return nil, nil
	}

	// Collect idTags
	idTagSet := make(map[string]bool)
	for _, tag := range tags {
		idTagSet[tag.IdTag] = true
	}

	// Find active transactions (not finished) that match user's tags
	states := make([]*entity.ChargeState, 0)
	for txId, tx := range db.transactions {
		if idTagSet[tx.IdTag] && !tx.IsFinished {
			if state, exists := db.chargeStates[txId]; exists {
				states = append(states, state)
			}
		}
	}
	return states, nil
}

func (db *MockDB) GetTransactions(_ context.Context, userId string, period string) ([]*entity.Transaction, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()

	// Get user's tags
	tags, ok := db.userTags[userId]
	if !ok || len(tags) == 0 {
		return nil, nil
	}

	// Collect idTags
	idTagSet := make(map[string]bool)
	for _, tag := range tags {
		idTagSet[tag.IdTag] = true
	}

	// Find transactions that match user's tags
	txs := make([]*entity.Transaction, 0)
	for _, tx := range db.transactions {
		if idTagSet[tx.IdTag] {
			txs = append(txs, tx)
		}
	}
	return txs, nil
}

func (db *MockDB) GetFilteredTransactions(_ context.Context, filter *entity.TransactionFilter) ([]*entity.Transaction, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()

	// Build set of id_tags to filter by username
	var usernameIdTags map[string]bool
	if filter.Username != "" {
		usernameIdTags = make(map[string]bool)
		for _, tags := range db.userTags {
			for _, tag := range tags {
				if tag.Username == filter.Username {
					usernameIdTags[tag.IdTag] = true
				}
			}
		}
		if len(usernameIdTags) == 0 {
			return []*entity.Transaction{}, nil
		}
	}

	txs := make([]*entity.Transaction, 0)
	for _, tx := range db.transactions {
		// Only finished transactions
		if !tx.IsFinished {
			continue
		}

		// Filter by username (via id_tags)
		if usernameIdTags != nil && !usernameIdTags[tx.IdTag] {
			continue
		}

		// Filter by id_tag
		if filter.IdTag != "" && tx.IdTag != filter.IdTag {
			continue
		}

		// Filter by charge_point_id
		if filter.ChargePointId != "" && tx.ChargePointId != filter.ChargePointId {
			continue
		}

		// Filter by date range
		if filter.From != nil && tx.TimeStart.Before(*filter.From) {
			continue
		}
		if filter.To != nil && tx.TimeStart.After(*filter.To) {
			continue
		}

		txs = append(txs, tx)
	}

	return txs, nil
}

func (db *MockDB) GetTransactionsToBill(userId string) ([]*entity.Transaction, error) {
	return nil, nil
}

func (db *MockDB) UpdateTransaction(transaction *entity.Transaction) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.transactions[transaction.TransactionId] = transaction
	return nil
}

// --- Meter Values ---

func (db *MockDB) GetLastMeterValue(_ context.Context, transactionId int) (*entity.TransactionMeter, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	values, ok := db.meterValues[transactionId]
	if !ok || len(values) == 0 {
		return nil, nil
	}
	return &values[len(values)-1], nil
}

func (db *MockDB) GetMeterValues(_ context.Context, transactionId int, from time.Time) ([]entity.TransactionMeter, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	values, ok := db.meterValues[transactionId]
	if !ok {
		return nil, nil
	}
	result := make([]entity.TransactionMeter, 0)
	for _, v := range values {
		if v.Time.After(from) {
			result = append(result, v)
		}
	}
	return result, nil
}

// --- Payment Methods ---

func (db *MockDB) GetPaymentMethods(_ context.Context, userId string) ([]*entity.PaymentMethod, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	methods, ok := db.paymentMethods[userId]
	if !ok {
		return nil, nil
	}
	return methods, nil
}

func (db *MockDB) SavePaymentMethod(_ context.Context, paymentMethod *entity.PaymentMethod) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.paymentMethods[paymentMethod.UserId] = append(db.paymentMethods[paymentMethod.UserId], paymentMethod)
	return nil
}

func (db *MockDB) UpdatePaymentMethod(_ context.Context, paymentMethod *entity.PaymentMethod) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	methods := db.paymentMethods[paymentMethod.UserId]
	for i, m := range methods {
		if m.Identifier == paymentMethod.Identifier {
			methods[i] = paymentMethod
			break
		}
	}
	return nil
}

func (db *MockDB) DeletePaymentMethod(_ context.Context, paymentMethod *entity.PaymentMethod) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	methods := db.paymentMethods[paymentMethod.UserId]
	for i, m := range methods {
		if m.Identifier == paymentMethod.Identifier {
			db.paymentMethods[paymentMethod.UserId] = append(methods[:i], methods[i+1:]...)
			break
		}
	}
	return nil
}

// --- Payment Orders ---

func (db *MockDB) GetLastOrder(_ context.Context) (*entity.PaymentOrder, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	if db.lastOrderId == 0 {
		return nil, nil
	}
	return db.paymentOrders[db.lastOrderId], nil
}

func (db *MockDB) SavePaymentOrder(_ context.Context, order *entity.PaymentOrder) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.paymentOrders[order.Order] = order
	if order.TransactionId > 0 {
		db.ordersByTx[order.TransactionId] = order
	}
	if order.Order > db.lastOrderId {
		db.lastOrderId = order.Order
	}
	return nil
}

func (db *MockDB) GetPaymentOrder(_ context.Context, id int) (*entity.PaymentOrder, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	order, ok := db.paymentOrders[id]
	if !ok {
		return nil, nil
	}
	return order, nil
}

func (db *MockDB) GetPaymentOrderByTransaction(_ context.Context, transactionId int) (*entity.PaymentOrder, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	order, ok := db.ordersByTx[transactionId]
	if !ok {
		return nil, nil
	}
	return order, nil
}

func (db *MockDB) SavePaymentResult(paymentParameters *entity.PaymentParameters) error {
	return nil
}

func (db *MockDB) GetPaymentParameters(orderId string) (*entity.PaymentParameters, error) {
	return nil, nil
}

// --- Locations and Charge Points ---

func (db *MockDB) ReadLog(_ context.Context, logName string) (interface{}, error) {
	return nil, nil
}

func (db *MockDB) ReadLogAfter(_ context.Context, timeStart time.Time) ([]*entity.FeatureMessage, error) {
	return nil, nil
}

func (db *MockDB) GetConfig(_ context.Context, name string) (interface{}, error) {
	return nil, nil
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

func (db *MockDB) GetRecentUserChargePoints(_ context.Context, userId string) ([]*entity.ChargePoint, error) {
	return nil, nil
}

// --- Reports ---

func (db *MockDB) TotalsByMonth(_ context.Context, from, to time.Time, userGroup string) ([]interface{}, error) {
	return nil, nil
}

func (db *MockDB) TotalsByUsers(_ context.Context, from, to time.Time, userGroup string) ([]interface{}, error) {
	return nil, nil
}

func (db *MockDB) TotalsByCharger(_ context.Context, from, to time.Time, userGroup string) ([]interface{}, error) {
	return nil, nil
}

func (db *MockDB) StationUptime(_ context.Context, from, to time.Time, chargePointId string) ([]*entity.StationUptime, error) {
	return nil, nil
}

func (db *MockDB) StationStatus(_ context.Context, chargePointId string) ([]*entity.StationStatus, error) {
	return nil, nil
}

// --- Preauthorizations ---

func (db *MockDB) SavePreauthorization(_ context.Context, preauth *entity.Preauthorization) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	db.preauthorizations[preauth.OrderNumber] = preauth
	if preauth.TransactionId > 0 {
		db.preauthByTx[preauth.TransactionId] = preauth
	}
	return nil
}

func (db *MockDB) GetPreauthorization(_ context.Context, orderNumber string) (*entity.Preauthorization, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	preauth, ok := db.preauthorizations[orderNumber]
	if !ok {
		return nil, nil
	}
	return preauth, nil
}

func (db *MockDB) GetPreauthorizationByTransaction(_ context.Context, transactionId int) (*entity.Preauthorization, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	preauth, ok := db.preauthByTx[transactionId]
	if !ok {
		return nil, nil
	}
	return preauth, nil
}

func (db *MockDB) UpdatePreauthorization(_ context.Context, preauth *entity.Preauthorization) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	existing, ok := db.preauthorizations[preauth.OrderNumber]
	if !ok {
		return fmt.Errorf("preauthorization not found")
	}
	existing.AuthorizationCode = preauth.AuthorizationCode
	existing.CapturedAmount = preauth.CapturedAmount
	existing.Status = preauth.Status
	existing.ErrorCode = preauth.ErrorCode
	existing.ErrorMessage = preauth.ErrorMessage
	existing.MerchantData = preauth.MerchantData
	existing.UpdatedAt = preauth.UpdatedAt
	return nil
}

func (db *MockDB) GetLastPreauthorizationOrder(_ context.Context) (*entity.Preauthorization, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	var latest *entity.Preauthorization
	for _, preauth := range db.preauthorizations {
		if latest == nil || preauth.CreatedAt.After(latest.CreatedAt) {
			latest = preauth
		}
	}
	return latest, nil
}
