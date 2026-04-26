package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	MaxAccessLevel              int = 10
	NormalizedMeterValuesLength     = 60
	subSystemReports                = "reports"
	maxRetryAttempts                = 4
)

var retryDelays = []time.Duration{
	1 * time.Hour,
	24 * time.Hour,
	7 * 24 * time.Hour,
	30 * 24 * time.Hour,
}

type Core struct {
	repo                 Repository
	auth                 Authenticator
	cs                   CentralSystem
	reports              Reports
	redsys               RedsysClient
	mail                 MailService
	currency             string
	disablePayment       bool
	paymentLocks         sync.Map
	stopPaymentProcessor chan struct{}
	log                  *slog.Logger
}

// MailService is the optional dependency that sends report emails on demand.
type MailService interface {
	SendNow(ctx context.Context, sub *entity.MailSubscription) error
	SendTest(ctx context.Context, to string) error
}

// RedsysClient interface for Redsys operations
type RedsysClient interface {
	Capture(ctx context.Context, req CaptureRequest) (*CaptureResponse, error)
	Cancel(ctx context.Context, req CaptureRequest) (*CaptureResponse, error)
	Preauthorize(ctx context.Context, req PreauthorizeRequest) (*CaptureResponse, error)
	Pay(ctx context.Context, req PayRequest) (*CaptureResponse, error)
	Refund(ctx context.Context, req RefundRequest) (*CaptureResponse, error)
	// BuildEntryForm signs a Redsys TPV Virtual hosted-form payload for
	// the web "add card" redirect flow. Used by POST /payment/order
	// when mode == "web".
	BuildEntryForm(req EntryFormRequest) (*EntryFormResponse, error)
}

// EntryFormRequest asks the Redsys adapter for a signed hosted-form
// payload. UrlOk / UrlKo come from the browser (so the same backend
// can serve multiple frontends / environments).
type EntryFormRequest struct {
	OrderNumber string
	Amount      int
	Description string
	UrlOk       string
	UrlKo       string
	Language    string
}

// EntryFormResponse is what the frontend needs to auto-submit an HTML
// form to the Redsys TPV Virtual entry point.
type EntryFormResponse struct {
	FormUrl            string
	SignatureVersion   string
	MerchantParameters string
	Signature          string
}

// CaptureRequest for Redsys capture operations
type CaptureRequest struct {
	OrderNumber       string
	Amount            int
	AuthorizationCode string
}

// CaptureResponse from Redsys operations
type CaptureResponse struct {
	Success           bool
	ResponseCode      string
	AuthorizationCode string
	ErrorCode         string
	ErrorMessage      string
	// Extended fields populated by Pay/Refund for payment processing
	MerchantIdentifier string
	CofTxnid           string
	CardBrand          string
	CardCountry        string
	ExpiryDate         string
	Order              string
	Amount             string
	Currency           string
	TransactionType    string
	Date               string
	Hour               string
}

// PreauthorizeRequest for Redsys MIT preauthorization
type PreauthorizeRequest struct {
	OrderNumber string
	Amount      int
	CardToken   string
	CofTid      string // Network transaction ID from initial authorization
}

// PayRequest for Redsys MIT direct payment (transaction type "0")
type PayRequest struct {
	OrderNumber string
	Amount      int
	CardToken   string // DS_MERCHANT_IDENTIFIER
	CofTid      string // DS_MERCHANT_COF_TXNID
}

// RefundRequest for Redsys refund (transaction type "3")
type RefundRequest struct {
	OrderNumber string
	Amount      int
}

func New(log *slog.Logger, repo Repository) *Core {
	return &Core{
		repo: repo,
		log:  log.With(sl.Module("impl.core")),
	}
}

func (c *Core) SetAuth(auth Authenticator) {
	c.auth = auth
}

func (c *Core) SetCentralSystem(cs CentralSystem) {
	c.cs = cs
}

func (c *Core) SetReports(reports Reports) {
	c.reports = reports
}

func (c *Core) SetRedsys(redsys RedsysClient) {
	c.redsys = redsys
}

func (c *Core) SetCurrency(currency string) {
	c.currency = currency
}

func (c *Core) SetDisablePayment(disable bool) {
	c.disablePayment = disable
}

func (c *Core) SetMailService(m MailService) {
	c.mail = m
}

// ListMailSubscriptions returns all mail subscriptions (admin only).
func (c *Core) ListMailSubscriptions(ctx context.Context, author *entity.User) ([]*entity.MailSubscription, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	return c.repo.ListMailSubscriptions(ctx)
}

// SaveMailSubscription creates or updates a mail subscription (admin only).
func (c *Core) SaveMailSubscription(ctx context.Context, author *entity.User, sub *entity.MailSubscription) (*entity.MailSubscription, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	return c.repo.SaveMailSubscription(ctx, sub)
}

// DeleteMailSubscription removes a mail subscription (admin only).
func (c *Core) DeleteMailSubscription(ctx context.Context, author *entity.User, id string) error {
	if err := c.requirePowerUser(author); err != nil {
		return err
	}
	return c.repo.DeleteMailSubscription(ctx, id)
}

// SendTestMail dispatches a minimal diagnostic email to verify Brevo
// credentials and sender verification. Admin-only.
func (c *Core) SendTestMail(ctx context.Context, author *entity.User, to string) error {
	if err := c.requirePowerUser(author); err != nil {
		return err
	}
	if c.mail == nil {
		return fmt.Errorf("mail service not configured")
	}
	if to == "" {
		return fmt.Errorf("recipient email is required")
	}
	return c.mail.SendTest(ctx, to)
}

// SendMailSubscriptionNow triggers an immediate report email for a subscription.
func (c *Core) SendMailSubscriptionNow(ctx context.Context, author *entity.User, id string) error {
	if err := c.requirePowerUser(author); err != nil {
		return err
	}
	if c.mail == nil {
		return fmt.Errorf("mail service not configured")
	}
	sub, err := c.repo.GetMailSubscription(ctx, id)
	if err != nil {
		return err
	}
	if sub == nil {
		return fmt.Errorf("subscription not found")
	}
	return c.mail.SendNow(ctx, sub)
}

// normalizeOrderNumber pads order number to 12 digits with leading zeros
func normalizeOrderNumber(orderNumber string) string {
	var orderNum int
	_, _ = fmt.Sscanf(orderNumber, "%d", &orderNum)
	return fmt.Sprintf("%012d", orderNum)
}

func (c *Core) requireAuth() error {
	if c.auth == nil {
		return fmt.Errorf("authenticator not set")
	}
	return nil
}

func (c *Core) requirePowerUser(author *entity.User) error {
	if c.auth == nil {
		return fmt.Errorf("authenticator not set")
	}
	if !author.IsPowerUser() {
		return fmt.Errorf("access denied: insufficient permissions")
	}
	return nil
}

func clearPassword(user *entity.User) *entity.User {
	if user != nil {
		user.Password = ""
	}
	return user
}

func (c *Core) GetConfig(ctx context.Context, name string) (interface{}, error) {
	return c.repo.GetConfig(ctx, name)
}

func (c *Core) GetLog(ctx context.Context, name string) (interface{}, error) {
	return c.repo.ReadLog(ctx, name)
}

func (c *Core) AuthenticateByToken(ctx context.Context, token string) (*entity.User, error) {
	if err := c.requireAuth(); err != nil {
		return nil, err
	}
	user, err := c.auth.AuthenticateByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	return clearPassword(user), nil
}

func (c *Core) AuthenticateUser(ctx context.Context, username, password string) (*entity.User, error) {
	if err := c.requireAuth(); err != nil {
		return nil, err
	}
	user, err := c.auth.AuthenticateUser(ctx, username, password)
	if err != nil {
		return nil, err
	}
	return clearPassword(user), nil
}

func (c *Core) AddUser(ctx context.Context, user *entity.User) (*entity.User, error) {
	if err := c.requireAuth(); err != nil {
		return nil, err
	}
	err := c.auth.RegisterUser(ctx, user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (c *Core) GetUser(ctx context.Context, author *entity.User, username string) (*entity.UserInfo, error) {
	if author.AccessLevel < 10 || username == "0000" { // user can get info about himself
		return c.repo.GetUserInfo(ctx, author.AccessLevel, author.Username)
	}
	return c.repo.GetUserInfo(ctx, author.AccessLevel, username)
}

func (c *Core) GetUsers(ctx context.Context, user *entity.User) ([]*entity.User, error) {
	if err := c.requireAuth(); err != nil {
		return nil, err
	}
	return c.auth.GetUsers(ctx, user)
}

func (c *Core) CreateUser(ctx context.Context, author *entity.User, user *entity.User) (*entity.User, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	err := c.auth.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	return clearPassword(user), nil
}

func (c *Core) UpdateUser(ctx context.Context, author *entity.User, username string, updates *entity.UserUpdate) (*entity.User, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	updated, err := c.auth.UpdateUser(ctx, username, updates)
	if err != nil {
		return nil, err
	}
	return clearPassword(updated), nil
}

func (c *Core) DeleteUser(ctx context.Context, author *entity.User, username string) error {
	if err := c.requirePowerUser(author); err != nil {
		return err
	}
	return c.auth.DeleteUser(ctx, username)
}

func (c *Core) UserTag(ctx context.Context, user *entity.User) (string, error) {
	if err := c.requireAuth(); err != nil {
		return "", err
	}
	if user == nil {
		return "", fmt.Errorf("user is nil")
	}
	return c.auth.GetUserTag(ctx, user)
}

func (c *Core) ListUserTags(ctx context.Context, author *entity.User) ([]*entity.UserTag, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	return c.auth.ListUserTags(ctx)
}

func (c *Core) GetUserTag(ctx context.Context, author *entity.User, idTag string) (*entity.UserTag, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	return c.auth.GetUserTagByIdTag(ctx, idTag)
}

func (c *Core) CreateUserTag(ctx context.Context, author *entity.User, tag *entity.UserTagCreate) (*entity.UserTag, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	return c.auth.CreateUserTag(ctx, tag)
}

func (c *Core) UpdateUserTag(ctx context.Context, author *entity.User, idTag string, updates *entity.UserTagUpdate) (*entity.UserTag, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	return c.auth.UpdateUserTag(ctx, idTag, updates)
}

func (c *Core) DeleteUserTag(ctx context.Context, author *entity.User, idTag string) error {
	if err := c.requirePowerUser(author); err != nil {
		return err
	}
	return c.auth.DeleteUserTag(ctx, idTag)
}

func (c *Core) GetLocations(ctx context.Context, accessLevel int) (interface{}, error) {
	if accessLevel < MaxAccessLevel {
		return nil, fmt.Errorf("access denied")
	}
	return c.repo.GetLocations(ctx)
}

func (c *Core) GetChargePoints(ctx context.Context, accessLevel int, search string) (interface{}, error) {
	list, err := c.repo.GetChargePoints(ctx, accessLevel, search)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return nil, nil
	}
	// disable connectors if charge point is not operational
	for _, cp := range list {
		cp.CheckConnectorsStatus()
		if accessLevel < MaxAccessLevel {
			cp.HideSensitiveData()
		}
	}
	return list, nil
}

func (c *Core) GetChargePoint(ctx context.Context, accessLevel int, id string) (interface{}, error) {
	cp, err := c.repo.GetChargePoint(ctx, accessLevel, id)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, nil
	}
	// disable connectors if charge point is not operational
	cp.CheckConnectorsStatus()
	if accessLevel < MaxAccessLevel {
		cp.HideSensitiveData()
	}
	return cp, nil
}

func (c *Core) SaveChargePoint(ctx context.Context, accessLevel int, chargePoint *entity.ChargePoint) error {
	if chargePoint == nil {
		return fmt.Errorf("charge point is nil")
	}
	if chargePoint.AccessLevel > MaxAccessLevel {
		chargePoint.AccessLevel = MaxAccessLevel
	}
	if accessLevel < chargePoint.AccessLevel {
		return fmt.Errorf("access denied")
	}
	return c.repo.UpdateChargePoint(ctx, accessLevel, chargePoint)
}

func (c *Core) SendCommand(command *entity.CentralSystemCommand, user *entity.User) (interface{}, error) {
	if c.cs == nil {
		return nil, fmt.Errorf("central system not set")
	}
	if command == nil {
		return nil, fmt.Errorf("command is nil")
	}
	err := c.auth.CommandAccess(user, command.FeatureName)
	if err != nil {
		return nil, err
	}
	return c.cs.SendCommand(command), nil
}

func (c *Core) GetActiveTransactions(ctx context.Context, userId string) (interface{}, error) {
	transactions, err := c.repo.GetActiveTransactions(ctx, userId)
	if err != nil {
		return nil, err
	}
	if transactions == nil {
		empty := make([]*entity.ChargeState, 0)
		return empty, nil
	}
	for _, t := range transactions {
		t.CheckState()
		t.MeterValues = NormalizeMeterValues(t.MeterValues, NormalizedMeterValuesLength)
	}
	return transactions, nil
}

func (c *Core) GetTransactions(ctx context.Context, userId, period string) (interface{}, error) {
	transactions, err := c.repo.GetTransactions(ctx, userId, period)
	if err != nil {
		return nil, err
	}
	if transactions == nil {
		empty := make([]*entity.Transaction, 0)
		return empty, nil
	}
	for _, t := range transactions {
		t.MeterValues = NormalizeMeterValues(t.MeterValues, NormalizedMeterValuesLength)
	}
	return transactions, nil
}

func (c *Core) GetFilteredTransactions(ctx context.Context, user *entity.User, filter *entity.TransactionFilter) (interface{}, error) {
	if !user.IsPowerUser() {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}
	transactions, err := c.repo.GetFilteredTransactions(ctx, filter)
	if err != nil {
		return nil, err
	}
	if transactions == nil {
		empty := make([]*entity.Transaction, 0)
		return empty, nil
	}
	for _, t := range transactions {
		t.MeterValues = NormalizeMeterValues(t.MeterValues, NormalizedMeterValuesLength)
	}
	return transactions, nil
}

func (c *Core) GetTransaction(ctx context.Context, userId string, accessLevel, id int) (interface{}, error) {
	state, err := c.repo.GetTransactionState(ctx, userId, accessLevel, id)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	state.CheckState()
	state.MeterValues = NormalizeMeterValues(state.MeterValues, NormalizedMeterValuesLength)
	return state, nil
}

func (c *Core) GetRecentChargePoints(ctx context.Context, userId string) (interface{}, error) {
	return c.repo.GetRecentUserChargePoints(ctx, userId)
}

func (c *Core) GetPaymentMethods(ctx context.Context, userId string) (interface{}, error) {
	return c.repo.GetPaymentMethods(ctx, userId)
}

func (c *Core) SavePaymentMethod(ctx context.Context, user *entity.User, pm *entity.PaymentMethod) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	if pm == nil {
		return fmt.Errorf("payment method is nil")
	}
	pm.UserId = user.UserId
	pm.UserName = user.Username
	return c.repo.SavePaymentMethod(ctx, pm)
}

func (c *Core) UpdatePaymentMethod(ctx context.Context, user *entity.User, pm *entity.PaymentMethod) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	if pm == nil {
		return fmt.Errorf("payment method is nil")
	}
	pm.UserId = user.UserId
	pm.UserName = user.Username
	return c.repo.UpdatePaymentMethod(ctx, pm)
}

func (c *Core) DeletePaymentMethod(ctx context.Context, user *entity.User, pm *entity.PaymentMethod) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	if pm == nil {
		return fmt.Errorf("payment method is nil")
	}
	// delete only methods, belonging to user that requested deletion
	pm.UserId = user.UserId
	return c.repo.DeletePaymentMethod(ctx, pm)
}

// WebOrderRequest carries the dynamic inputs for CreateWebOrder — the
// browser-provided return URLs and the language tag used by Redsys.
type WebOrderRequest struct {
	UrlOk    string
	UrlKo    string
	Language string
}

// CreateWebOrder persists a new PaymentOrder (via SetOrder) and returns
// the signed Redsys TPV Virtual hosted-form payload the browser needs
// to POST to the Redsys entry URL for the "add card" redirect flow.
//
// After the user completes card entry on Redsys, Redsys delivers the
// permanent card token server-to-server to POST /payment/notify, which
// the existing Notify → processNotifyResponseAsync → processPaymentResponse
// chain already handles: orders with no TransactionId are treated as
// card enrollments and a fresh PaymentMethod is stored for the user.
//
// The Android/native flow does NOT go through this path and keeps
// using SetOrder directly.
func (c *Core) CreateWebOrder(ctx context.Context, user *entity.User, order *entity.PaymentOrder, req *WebOrderRequest) (*entity.PaymentOrder, *EntryFormResponse, error) {
	if c.redsys == nil {
		return nil, nil, fmt.Errorf("redsys client not configured")
	}
	if req == nil || req.UrlOk == "" || req.UrlKo == "" {
		return nil, nil, fmt.Errorf("url_ok and url_ko are required")
	}

	// Delegate order creation (auto-numbering, interrupted-order cleanup,
	// user binding, persistence) to the existing SetOrder logic.
	stored, err := c.SetOrder(ctx, user, order)
	if err != nil {
		return nil, nil, err
	}

	form, err := c.redsys.BuildEntryForm(EntryFormRequest{
		OrderNumber: normalizeOrderNumber(fmt.Sprintf("%d", stored.Order)),
		Amount:      stored.Amount,
		Description: stored.Description,
		UrlOk:       req.UrlOk,
		UrlKo:       req.UrlKo,
		Language:    req.Language,
	})
	if err != nil {
		return stored, nil, fmt.Errorf("build entry form: %w", err)
	}
	return stored, form, nil
}

func (c *Core) SetOrder(ctx context.Context, user *entity.User, order *entity.PaymentOrder) (*entity.PaymentOrder, error) {
	if user == nil {
		return nil, fmt.Errorf("user is nil")
	}
	if order == nil {
		return nil, fmt.Errorf("order is nil")
	}
	if order.Order == 0 {

		// if payment process was interrupted, find unclosed order and close it
		if order.TransactionId > 0 {
			continueOrder, _ := c.repo.GetPaymentOrderByTransaction(ctx, order.TransactionId)
			if continueOrder != nil {
				continueOrder.IsCompleted = true
				continueOrder.Result = "closed without response"
				continueOrder.TimeClosed = time.Now()
				_ = c.repo.SavePaymentOrder(ctx, continueOrder)
			}
		}

		lastOrder, _ := c.repo.GetLastOrder(ctx)
		if lastOrder != nil {
			order.Order = lastOrder.Order + 1
		} else {
			order.Order = 1200
		}
		order.TimeOpened = time.Now()
	}

	order.UserId = user.UserId
	order.UserName = user.Username

	err := c.repo.SavePaymentOrder(ctx, order)
	if err != nil {
		return nil, err
	}
	return order, nil
}

// WsRequest handler for requests made via websocket
func (c *Core) WsRequest(request *entity.UserRequest) error {
	if c.cs == nil {
		return fmt.Errorf("central system is not connected")
	}

	var command *entity.CentralSystemCommand

	switch request.Command {
	case entity.StartTransaction:
		command = entity.NewCommandStartTransaction(request.ChargePointId, request.ConnectorId, request.Token)
	case entity.StopTransaction:
		command = entity.NewCommandStopTransaction(request.ChargePointId, request.ConnectorId, request.TransactionId)
	case entity.CheckStatus:
		return nil
	case entity.ListenTransaction:
		return nil
	case entity.StopListenTransaction:
		return nil
	case entity.ListenChargePoints:
		return nil
	case entity.ListenLog:
		return nil
	case entity.PingConnection:
		return nil
	default:
		return fmt.Errorf("unknown command %s", request.Command)
	}

	response := c.cs.SendCommand(command)
	if response.IsError() {
		return fmt.Errorf("sending command to central system: %s", response.Info)
	}

	return nil
}

func (c *Core) MonthlyStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]interface{}, error) {
	err := c.checkSubsystemAccess(user, subSystemReports)
	if err != nil {
		return nil, err
	}
	return c.reports.TotalsByMonth(ctx, from, to, userGroup)
}

func (c *Core) UsersStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]interface{}, error) {
	err := c.checkSubsystemAccess(user, subSystemReports)
	if err != nil {
		return nil, err
	}
	return c.reports.TotalsByUsers(ctx, from, to, userGroup)
}

func (c *Core) ChargerStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]interface{}, error) {
	err := c.checkSubsystemAccess(user, subSystemReports)
	if err != nil {
		return nil, err
	}
	return c.reports.TotalsByCharger(ctx, from, to, userGroup)
}

func (c *Core) ExportStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]interface{}, error) {
	err := c.checkSubsystemAccess(user, subSystemReports)
	if err != nil {
		return nil, err
	}
	return c.reports.TotalsByHour(ctx, from, to, userGroup)
}

func (c *Core) StationUptimeReport(ctx context.Context, user *entity.User, from, to time.Time, chargePointId string) ([]*entity.StationUptime, error) {
	err := c.checkSubsystemAccess(user, subSystemReports)
	if err != nil {
		return nil, err
	}
	return c.reports.StationUptime(ctx, from, to, chargePointId)
}

func (c *Core) StationStatusReport(ctx context.Context, user *entity.User, chargePointId string) ([]*entity.StationStatus, error) {
	err := c.checkSubsystemAccess(user, subSystemReports)
	if err != nil {
		return nil, err
	}
	return c.reports.StationStatus(ctx, chargePointId)
}

// CreatePreauthorizationOrder creates a new preauthorization order and performs MIT preauthorization via Redsys
func (c *Core) CreatePreauthorizationOrder(ctx context.Context, user *entity.User, req *entity.PreauthorizationOrderRequest) (*entity.PreauthorizationOrderResponse, error) {
	if user == nil {
		return nil, fmt.Errorf("user is nil")
	}
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if c.redsys == nil {
		return nil, fmt.Errorf("redsys client not configured")
	}

	// Generate order number based on last order
	lastOrder, _ := c.repo.GetLastPreauthorizationOrder(ctx)
	var orderNum int
	if lastOrder != nil {
		// Parse last order number and increment
		_, _ = fmt.Sscanf(lastOrder.OrderNumber, "%d", &orderNum)
		orderNum++
	} else {
		orderNum = 3000 // Start from 3000
	}
	orderNumber := fmt.Sprintf("%012d", orderNum)

	// Default transaction type to "1" (preauthorization) if not specified
	transactionType := req.TransactionType
	if transactionType == "" {
		transactionType = "1"
	}

	// Use currency from config, default to EUR (978) if not set
	currency := c.currency
	if currency == "" {
		currency = "978"
	}

	now := time.Now()
	preauth := &entity.Preauthorization{
		OrderNumber:         orderNumber,
		PreauthorizedAmount: req.Amount,
		Status:              entity.PreauthorizationStatusPending,
		TransactionId:       req.TransactionId,
		PaymentMethodId:     req.PaymentMethodId,
		UserId:              user.UserId,
		UserName:            user.Username,
		Currency:            currency,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	// Save initial preauthorization record
	err := c.repo.SavePreauthorization(ctx, preauth)
	if err != nil {
		return nil, fmt.Errorf("failed to save preauthorization: %w", err)
	}

	// Get payment method to retrieve CofTid for MIT transaction
	var cofTid string
	paymentMethods, err := c.repo.GetPaymentMethods(ctx, user.UserId)
	if err != nil {
		c.log.With(sl.Err(err)).Warn("failed to get payment methods for CofTid lookup")
	} else if paymentMethods != nil {
		for _, pm := range paymentMethods {
			if pm.Identifier == req.PaymentMethodId {
				cofTid = pm.CofTid
				break
			}
		}
	}

	// Call Redsys REST API for MIT preauthorization
	preauthorizeReq := PreauthorizeRequest{
		OrderNumber: orderNumber,
		Amount:      req.Amount,
		CardToken:   req.PaymentMethodId,
		CofTid:      cofTid,
	}

	resp, err := c.redsys.Preauthorize(ctx, preauthorizeReq)
	if err != nil {
		// Update preauthorization with error
		preauth.Status = entity.PreauthorizationStatusFailed
		preauth.ErrorMessage = err.Error()
		preauth.UpdatedAt = time.Now()
		if updateErr := c.repo.UpdatePreauthorization(ctx, preauth); updateErr != nil {
			c.log.With(sl.Err(updateErr)).Error("failed to update preauthorization after error")
		}
		return &entity.PreauthorizationOrderResponse{
			Order:           orderNum,
			Amount:          req.Amount,
			Description:     req.Description,
			TransactionType: transactionType,
			PaymentMethodId: req.PaymentMethodId,
			TransactionId:   req.TransactionId,
			Error:           err.Error(),
		}, nil
	}

	// Update preauthorization based on Redsys response
	if resp.Success {
		preauth.Status = entity.PreauthorizationStatusAuthorized
		preauth.AuthorizationCode = resp.AuthorizationCode
	} else {
		preauth.Status = entity.PreauthorizationStatusFailed
		preauth.ErrorCode = resp.ErrorCode
		preauth.ErrorMessage = resp.ErrorMessage
	}
	preauth.UpdatedAt = time.Now()

	if updateErr := c.repo.UpdatePreauthorization(ctx, preauth); updateErr != nil {
		c.log.With(sl.Err(updateErr)).Error("failed to update preauthorization after Redsys response")
	}

	response := &entity.PreauthorizationOrderResponse{
		Order:           orderNum,
		Amount:          req.Amount,
		Description:     req.Description,
		TransactionType: transactionType,
		PaymentMethodId: req.PaymentMethodId,
		TransactionId:   req.TransactionId,
	}

	if resp.Success {
		response.AuthorizationCode = resp.AuthorizationCode
	} else {
		response.Error = resp.ErrorMessage
		if response.Error == "" && resp.ErrorCode != "" {
			response.Error = fmt.Sprintf("Redsys error code: %s", resp.ErrorCode)
		}
	}

	return response, nil
}

// SavePreauthorization saves a preauthorization result from Redsys callback
func (c *Core) SavePreauthorization(ctx context.Context, user *entity.User, req *entity.PreauthorizationSaveRequest) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	// Normalize order number to 12-digit format
	orderNumber := normalizeOrderNumber(req.OrderNumber)
	preauth, err := c.repo.GetPreauthorization(ctx, orderNumber)
	if err != nil {
		return fmt.Errorf("failed to get preauthorization: %w", err)
	}
	if preauth == nil {
		return fmt.Errorf("preauthorization not found")
	}

	// Verify user owns this preauthorization
	if preauth.UserId != user.UserId {
		return fmt.Errorf("access denied: preauthorization belongs to another user")
	}

	// Update preauthorization based on result
	switch req.Status {
	case "AUTHORIZED":
		preauth.Status = entity.PreauthorizationStatusAuthorized
	case "FAILED":
		preauth.Status = entity.PreauthorizationStatusFailed
	case "CANCELLED":
		preauth.Status = entity.PreauthorizationStatusCancelled
	default:
		preauth.Status = entity.PreauthorizationStatusFailed
	}

	preauth.AuthorizationCode = req.AuthorizationCode
	preauth.ErrorCode = req.ErrorCode
	preauth.ErrorMessage = req.ErrorMessage
	preauth.MerchantData = req.MerchantData
	preauth.UpdatedAt = time.Now()

	return c.repo.UpdatePreauthorization(ctx, preauth)
}

// GetPreauthorization retrieves a preauthorization by transaction ID
func (c *Core) GetPreauthorization(ctx context.Context, user *entity.User, transactionId int) (*entity.Preauthorization, error) {
	if user == nil {
		return nil, fmt.Errorf("user is nil")
	}

	preauth, err := c.repo.GetPreauthorizationByTransaction(ctx, transactionId)
	if err != nil {
		return nil, fmt.Errorf("failed to get preauthorization: %w", err)
	}
	if preauth == nil {
		return nil, nil
	}

	// Verify user owns this preauthorization or is admin
	if preauth.UserId != user.UserId && !user.IsPowerUser() {
		return nil, fmt.Errorf("access denied: preauthorization belongs to another user")
	}

	return preauth, nil
}

// CapturePreauthorization captures a preauthorized amount via Redsys
func (c *Core) CapturePreauthorization(ctx context.Context, user *entity.User, req *entity.CaptureOrderRequest) (*entity.CaptureOrderResponse, error) {
	if user == nil {
		return nil, fmt.Errorf("user is nil")
	}
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if c.redsys == nil {
		return nil, fmt.Errorf("redsys client not configured")
	}

	// Normalize order number to 12-digit format
	orderNumber := normalizeOrderNumber(req.OriginalOrder)
	preauth, err := c.repo.GetPreauthorization(ctx, orderNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get preauthorization: %w", err)
	}
	if preauth == nil {
		return nil, fmt.Errorf("preauthorization not found")
	}

	// Verify preauthorization is in correct state
	if preauth.Status != entity.PreauthorizationStatusAuthorized {
		return nil, fmt.Errorf("preauthorization is not in authorized state")
	}

	// Verify capture amount doesn't exceed preauthorized amount
	if req.Amount > preauth.PreauthorizedAmount {
		return nil, fmt.Errorf("capture amount exceeds preauthorized amount")
	}

	// Use authorization code from request if provided, otherwise from stored preauth
	authCode := req.AuthorizationCode
	if authCode == "" {
		authCode = preauth.AuthorizationCode
	}

	// Call Redsys to capture - use normalized order number
	captureReq := CaptureRequest{
		OrderNumber:       orderNumber,
		Amount:            req.Amount,
		AuthorizationCode: authCode,
	}

	resp, err := c.redsys.Capture(ctx, captureReq)
	if err != nil {
		return nil, fmt.Errorf("failed to capture: %w", err)
	}

	// Parse order number for response
	var orderNum int
	_, _ = fmt.Sscanf(req.OriginalOrder, "%d", &orderNum)

	// Default transaction type to "2" (capture) if not specified
	transactionType := req.TransactionType
	if transactionType == "" {
		transactionType = "2"
	}

	// Update preauthorization based on result
	if resp.Success {
		preauth.Status = entity.PreauthorizationStatusCaptured
		preauth.CapturedAmount = req.Amount
	} else {
		preauth.ErrorCode = resp.ErrorCode
		preauth.ErrorMessage = resp.ErrorMessage
	}
	preauth.UpdatedAt = time.Now()

	if err := c.repo.UpdatePreauthorization(ctx, preauth); err != nil {
		c.log.With(sl.Err(err)).Error("failed to update preauthorization after capture")
	}

	return &entity.CaptureOrderResponse{
		Order:           orderNum,
		Amount:          req.Amount,
		TransactionType: transactionType,
		Status:          preauth.Status,
		ErrorCode:       resp.ErrorCode,
		ErrorMessage:    resp.ErrorMessage,
	}, nil
}

// UpdatePreauthorization updates preauthorization status
func (c *Core) UpdatePreauthorization(ctx context.Context, user *entity.User, req *entity.PreauthorizationUpdateRequest) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	// Normalize order number to 12-digit format
	orderNumber := normalizeOrderNumber(req.OrderNumber)
	preauth, err := c.repo.GetPreauthorization(ctx, orderNumber)
	if err != nil {
		return fmt.Errorf("failed to get preauthorization: %w", err)
	}
	if preauth == nil {
		return fmt.Errorf("preauthorization not found")
	}

	// Verify user owns this preauthorization or is admin
	if preauth.UserId != user.UserId && !user.IsPowerUser() {
		return fmt.Errorf("access denied: preauthorization belongs to another user")
	}

	switch req.Status {
	case "AUTHORIZED":
		preauth.Status = entity.PreauthorizationStatusAuthorized
	case "CAPTURED":
		preauth.Status = entity.PreauthorizationStatusCaptured
	case "CANCELLED":
		preauth.Status = entity.PreauthorizationStatusCancelled
	case "FAILED":
		preauth.Status = entity.PreauthorizationStatusFailed
	default:
		return fmt.Errorf("invalid status: %s", req.Status)
	}

	preauth.UpdatedAt = time.Now()
	return c.repo.UpdatePreauthorization(ctx, preauth)
}

// --- Direct Payment Methods (ported from electrum) ---

// lockOrder acquires a per-order mutex to prevent concurrent modifications.
func (c *Core) lockOrder(id int) *sync.Mutex {
	value, _ := c.paymentLocks.LoadOrStore(id, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex
}

// unlockOrder releases the per-order mutex and cleans up to prevent memory leaks.
func (c *Core) unlockOrder(id int, mutex *sync.Mutex) {
	mutex.Unlock()
	c.paymentLocks.Delete(id)
}

// PayTransaction initiates a direct MIT payment for a finished charging transaction.
func (c *Core) PayTransaction(ctx context.Context, transactionId int) error {
	mutex := c.lockOrder(transactionId)
	defer c.unlockOrder(transactionId, mutex)

	log := c.log.With(slog.Int("transaction_id", transactionId))
	log.Info("pay transaction")

	if c.redsys == nil {
		return fmt.Errorf("redsys client not configured")
	}

	transaction, err := c.repo.GetTransaction(ctx, transactionId)
	if err != nil {
		log.With(sl.Err(err)).Error("failed to get transaction")
		return err
	}
	if transaction == nil {
		return fmt.Errorf("transaction %d not found", transactionId)
	}
	if !transaction.IsFinished {
		return fmt.Errorf("transaction %d is not finished", transactionId)
	}

	amount := transaction.PaymentAmount - transaction.PaymentBilled
	if amount <= 0 {
		log.Warn("transaction amount is zero or already billed")
		return nil
	}

	// Resolve user tag
	tag := transaction.UserTag
	if tag == nil {
		tag, err = c.repo.GetUserTag(ctx, transaction.IdTag)
		if err != nil {
			log.With(sl.Err(err)).Error("failed to get user tag")
			return err
		}
	}
	if tag.UserId == "" {
		transaction.PaymentBilled = transaction.PaymentAmount
		if e := c.repo.UpdateTransactionPayment(ctx, transaction); e != nil {
			log.With(sl.Err(e)).Error("failed to update transaction")
		}
		return fmt.Errorf("empty user id for tag %s", tag.IdTag)
	}

	// Resolve payment method with fallback logic
	paymentMethod := transaction.PaymentMethod
	if paymentMethod == nil {
		paymentMethod, err = c.repo.GetDefaultPaymentMethod(ctx, tag.UserId)
		if err != nil {
			transaction.PaymentBilled = transaction.PaymentAmount
			if e := c.repo.UpdateTransactionPayment(ctx, transaction); e != nil {
				log.With(sl.Err(e)).Error("failed to update transaction")
			}
			return fmt.Errorf("no payment method for user %s", tag.UserId)
		}
	}
	// Try to get alternative if current has problems
	if paymentMethod.CofTid == "" || paymentMethod.FailCount > 0 || transaction.PaymentError != "" {
		storedPM, _ := c.repo.GetDefaultPaymentMethod(ctx, tag.UserId)
		if storedPM != nil && storedPM.Identifier != paymentMethod.Identifier {
			paymentMethod = storedPM
			log.With(sl.Secret("identifier", storedPM.Identifier)).Warn("switched to alternative payment method")
		}
	}

	// Close any previous uncompleted order for this transaction
	orderToClose, err := c.repo.GetPaymentOrderByTransaction(ctx, transactionId)
	if err == nil && orderToClose != nil {
		orderToClose.IsCompleted = true
		orderToClose.Result = "closed without response"
		orderToClose.TimeClosed = time.Now()
		if e := c.repo.SavePaymentOrder(ctx, orderToClose); e != nil {
			log.With(sl.Err(e)).Error("failed to close previous payment order")
		}
		c.updatePaymentMethodFailCounter(ctx, orderToClose.Identifier, 1)
	}

	// DisablePayment bypass for testing
	if c.disablePayment {
		transaction.PaymentBilled = transaction.PaymentAmount
		if e := c.repo.UpdateTransactionPayment(ctx, transaction); e != nil {
			log.With(sl.Err(e)).Error("failed to update transaction")
		}
		log.Info("payment disabled: transaction paid without request")
		return nil
	}

	// Create payment order
	consumed := (transaction.MeterStop - transaction.MeterStart) / 1000
	description := fmt.Sprintf("%s:%d %dkW", transaction.ChargePointId, transaction.ConnectorId, consumed)

	paymentOrder := entity.PaymentOrder{
		Amount:        amount,
		Description:   description,
		Identifier:    paymentMethod.Identifier,
		TransactionId: transactionId,
		UserId:        tag.UserId,
		UserName:      tag.Username,
		TimeOpened:    time.Now(),
	}

	lastOrder, _ := c.repo.GetLastOrder(ctx)
	if lastOrder != nil {
		paymentOrder.Order = lastOrder.Order + 1
	} else {
		paymentOrder.Order = 1200
	}

	if e := c.repo.SavePaymentOrder(ctx, &paymentOrder); e != nil {
		log.With(sl.Err(e)).Error("failed to save payment order")
		return e
	}

	orderNumber := fmt.Sprintf("%d", paymentOrder.Order)
	log.With(
		slog.String("order", orderNumber),
		sl.Secret("identifier", paymentMethod.Identifier),
		sl.Secret("cof_txnid", paymentMethod.CofTid),
	).Info("sending payment request")

	// Process payment asynchronously
	go c.processPayAsync(ctx, PayRequest{
		OrderNumber: orderNumber,
		Amount:      amount,
		CardToken:   paymentMethod.Identifier,
		CofTid:      paymentMethod.CofTid,
	}, paymentOrder.Order)

	return nil
}

// ReturnPayment processes a full refund for a charging transaction.
func (c *Core) ReturnPayment(ctx context.Context, transactionId int) error {
	mutex := c.lockOrder(transactionId)
	defer c.unlockOrder(transactionId, mutex)

	log := c.log.With(slog.Int("transaction_id", transactionId))

	if c.redsys == nil {
		return fmt.Errorf("redsys client not configured")
	}

	transaction, err := c.repo.GetTransaction(ctx, transactionId)
	if err != nil {
		log.With(sl.Err(err)).Error("failed to get transaction for refund")
		return err
	}
	if transaction == nil {
		return fmt.Errorf("transaction %d not found", transactionId)
	}
	if !transaction.IsFinished {
		return fmt.Errorf("transaction %d is not finished", transactionId)
	}

	amount := transaction.PaymentAmount
	if amount <= 0 {
		log.Warn("transaction amount is zero, nothing to refund")
		return nil
	}

	orderNumber := fmt.Sprintf("%d", transaction.PaymentOrder)

	go c.processRefundAsync(ctx, RefundRequest{
		OrderNumber: orderNumber,
		Amount:      amount,
	}, transaction.PaymentOrder)

	return nil
}

// ReturnByOrder processes a refund for a specific payment order.
func (c *Core) ReturnByOrder(ctx context.Context, orderId string, amount int) error {
	if amount == 0 {
		return fmt.Errorf("amount to return is zero")
	}

	id, err := strconv.Atoi(orderId)
	if err != nil {
		return fmt.Errorf("invalid order id: %s", orderId)
	}

	mutex := c.lockOrder(id)
	defer c.unlockOrder(id, mutex)

	if c.redsys == nil {
		return fmt.Errorf("redsys client not configured")
	}

	order, err := c.repo.GetPaymentOrder(ctx, id)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}
	if order == nil {
		return fmt.Errorf("payment order %d not found", id)
	}
	if order.Amount < amount {
		return fmt.Errorf("order amount %d is less than return amount %d", order.Amount, amount)
	}

	go c.processRefundAsync(ctx, RefundRequest{
		OrderNumber: orderId,
		Amount:      amount,
	}, id)

	return nil
}

// Notify processes a payment notification webhook from Redsys.
func (c *Core) Notify(ctx context.Context, data []byte) error {
	params, err := url.ParseQuery(string(data))
	if err != nil {
		c.log.With(slog.String("data", string(data))).Error("failed to parse notification data")
		return fmt.Errorf("parse query: %w", err)
	}

	merchantParams := params.Get("Ds_MerchantParameters")
	if merchantParams == "" {
		return fmt.Errorf("empty merchant parameters")
	}

	paymentResult, err := c.decodePaymentParameters(merchantParams)
	if err != nil {
		return err
	}

	go c.processNotifyResponseAsync(ctx, paymentResult)
	return nil
}

// processPayAsync sends a payment request to Redsys and processes the response.
func (c *Core) processPayAsync(parentCtx context.Context, req PayRequest, orderId int) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic in processPayAsync", slog.Any("panic", r))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log := c.log.With(slog.String("order", req.OrderNumber), slog.Int("amount", req.Amount))

	resp, err := c.redsys.Pay(ctx, req)
	if err != nil {
		log.With(sl.Err(err)).Error("payment request failed")
		order, _ := c.repo.GetPaymentOrder(ctx, orderId)
		if order != nil {
			c.closeOrderOnError(ctx, order, err.Error())
		}
		return
	}

	if !resp.Success {
		log.With(slog.String("error_code", resp.ErrorCode)).Warn("payment rejected by Redsys")
		order, _ := c.repo.GetPaymentOrder(ctx, orderId)
		if order != nil {
			c.closeOrderOnError(ctx, order, resp.ResponseCode)
		}
		return
	}

	c.processPaymentResponse(ctx, resp, orderId)
}

// processRefundAsync sends a refund request to Redsys and processes the response.
func (c *Core) processRefundAsync(parentCtx context.Context, req RefundRequest, orderId int) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic in processRefundAsync", slog.Any("panic", r))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log := c.log.With(slog.String("order", req.OrderNumber), slog.Int("amount", req.Amount))

	resp, err := c.redsys.Refund(ctx, req)
	if err != nil {
		log.With(sl.Err(err)).Error("refund request failed")
		return
	}

	if !resp.Success {
		log.With(slog.String("error_code", resp.ErrorCode)).Warn("refund rejected by Redsys")
		order, _ := c.repo.GetPaymentOrder(ctx, orderId)
		if order != nil {
			c.closeOrderOnError(ctx, order, resp.ResponseCode)
		}
		return
	}

	c.processPaymentResponse(ctx, resp, orderId)
}

// processNotifyResponseAsync handles async Redsys webhook notification.
func (c *Core) processNotifyResponseAsync(parentCtx context.Context, paymentResult *entity.PaymentParameters) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic in processNotifyResponseAsync", slog.Any("panic", r))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if e := c.repo.SavePaymentResult(ctx, paymentResult); e != nil {
		c.log.With(sl.Err(e)).Error("failed to save payment result")
	}

	orderNum, err := strconv.Atoi(paymentResult.Order)
	if err != nil {
		c.log.With(sl.Err(err)).Error("failed to parse order number from notification")
		return
	}

	// Convert PaymentParameters to a CaptureResponse-like structure for processPaymentResponse
	resp := &CaptureResponse{
		Success:            paymentResult.Response == "0000" || paymentResult.Response == "0900",
		ResponseCode:       paymentResult.Response,
		AuthorizationCode:  paymentResult.AuthorisationCode,
		MerchantIdentifier: paymentResult.MerchantIdentifier,
		CofTxnid:           paymentResult.MerchantCofTxnid,
		CardBrand:          paymentResult.CardBrand,
		CardCountry:        paymentResult.CardCountry,
		ExpiryDate:         paymentResult.ExpiryDate,
		Order:              paymentResult.Order,
		Amount:             paymentResult.Amount,
		Currency:           paymentResult.Currency,
		TransactionType:    paymentResult.TransactionType,
		Date:               paymentResult.Date,
		Hour:               paymentResult.Hour,
	}

	if !resp.Success {
		order, _ := c.repo.GetPaymentOrder(ctx, orderNum)
		if order != nil {
			c.closeOrderOnError(ctx, order, paymentResult.Response)
		}
		return
	}

	c.processPaymentResponse(ctx, resp, orderNum)
}

// processPaymentResponse handles a successful Redsys response: updates order, transaction billing, and payment methods.
func (c *Core) processPaymentResponse(ctx context.Context, resp *CaptureResponse, orderNum int) {
	log := c.log.With(slog.String("order", fmt.Sprintf("%d", orderNum)))

	amount, _ := strconv.Atoi(resp.Amount)

	order, err := c.repo.GetPaymentOrder(ctx, orderNum)
	if err != nil {
		log.With(sl.Err(err)).Error("failed to get payment order")
		return
	}
	if order == nil {
		log.Error("payment order not found")
		return
	}

	// Close the order
	if !order.IsCompleted {
		order.Amount = amount
		order.IsCompleted = true
		order.Result = resp.ResponseCode
		order.TimeClosed = time.Now()
		order.Currency = resp.Currency
		order.Date = fmt.Sprintf("%s %s", resp.Date, resp.Hour)

		if e := c.repo.SavePaymentOrder(ctx, order); e != nil {
			log.With(sl.Err(e)).Error("failed to save payment order")
		}
	}

	// Reset fail counter on success
	c.updatePaymentMethodFailCounter(ctx, order.Identifier, 0)

	// Handle refund response (transaction type "3")
	if resp.TransactionType == "3" {
		order.RefundAmount = amount
		order.RefundTime = time.Now()
		if e := c.repo.SavePaymentOrder(ctx, order); e != nil {
			log.With(sl.Err(e)).Error("failed to save refund order")
		}
		return
	}

	// Update transaction billing
	if order.TransactionId > 0 {
		transaction, e := c.repo.GetTransaction(ctx, order.TransactionId)
		if e != nil {
			log.With(sl.Err(e)).Error("failed to get transaction")
			return
		}
		if transaction == nil {
			log.Error("transaction not found for order")
			return
		}

		transaction.PaymentOrder = order.Order
		transaction.PaymentBilled = transaction.PaymentBilled + order.Amount
		transaction.PaymentError = ""
		transaction.AddOrder(*order)

		if e := c.repo.UpdateTransactionPayment(ctx, transaction); e != nil {
			log.With(sl.Err(e)).Error("failed to update transaction billing")
		}

		if e := c.repo.DeletePaymentRetry(ctx, order.TransactionId); e != nil {
			log.With(sl.Err(e)).Error("failed to delete payment retry record")
		}
	} else {
		// No transaction linked — this is a card enrollment response; save payment method
		pm := entity.PaymentMethod{
			Description: "**** **** **** ****",
			Identifier:  resp.MerchantIdentifier,
			CofTid:      resp.CofTxnid,
			CardBrand:   resp.CardBrand,
			CardCountry: resp.CardCountry,
			ExpiryDate:  resp.ExpiryDate,
			UserId:      order.UserId,
			UserName:    order.UserName,
		}
		if pm.Identifier != "" && pm.UserId != "" {
			if e := c.repo.SavePaymentMethod(ctx, &pm); e != nil {
				log.With(sl.Err(e)).Error("failed to save payment method from response")
			} else {
				log.With(sl.Secret("identifier", pm.Identifier)).Info("payment method saved")
			}
		}

		// Refund the enrollment charge
		if order.Amount > 0 {
			id := fmt.Sprintf("%d", order.Order)
			if e := c.ReturnByOrder(ctx, id, order.Amount); e != nil {
				log.With(sl.Err(e)).Error("failed to refund enrollment charge")
			}
		}
	}
}

// closeOrderOnError marks a payment order as failed and updates the related transaction.
func (c *Core) closeOrderOnError(ctx context.Context, order *entity.PaymentOrder, result string) {
	c.updatePaymentMethodFailCounter(ctx, order.Identifier, 1)

	if !order.IsCompleted {
		order.IsCompleted = true
		order.Result = result
		order.TimeClosed = time.Now()
		if e := c.repo.SavePaymentOrder(ctx, order); e != nil {
			c.log.With(sl.Err(e)).Error("failed to save payment order on error")
		}
	}

	if order.TransactionId > 0 {
		c.log.With(slog.Int("transaction_id", order.TransactionId)).Info("closing transaction on payment error")
		transaction, e := c.repo.GetTransaction(ctx, order.TransactionId)
		if e != nil {
			c.log.With(sl.Err(e)).Error("failed to get transaction")
			return
		}
		if transaction == nil {
			return
		}
		transaction.PaymentBilled = transaction.PaymentAmount
		transaction.PaymentOrder = order.Order
		transaction.PaymentError = result
		transaction.AddOrder(*order)
		if e := c.repo.UpdateTransactionPayment(ctx, transaction); e != nil {
			c.log.With(sl.Err(e)).Error("failed to update transaction on error")
		}

		c.schedulePaymentRetry(ctx, order.TransactionId, result)
	}
}

// updatePaymentMethodFailCounter updates the fail count for a payment method.
// count=0 resets, count>0 increments.
func (c *Core) updatePaymentMethodFailCounter(ctx context.Context, identifier string, count int) {
	if identifier == "" {
		return
	}

	pm, err := c.repo.GetPaymentMethodByIdentifier(ctx, identifier)
	if err != nil || pm == nil {
		return
	}

	newCount := count
	if count > 0 {
		newCount = pm.FailCount + 1
	}

	if e := c.repo.UpdatePaymentMethodFailCount(ctx, identifier, newCount); e != nil {
		c.log.With(sl.Err(e)).Error("failed to update payment method fail counter")
	}
}

// decodePaymentParameters decodes base64-encoded Redsys merchant parameters.
func (c *Core) decodePaymentParameters(encodedParams string) (*entity.PaymentParameters, error) {
	decoded, err := base64.StdEncoding.DecodeString(encodedParams)
	if err != nil {
		return nil, fmt.Errorf("decode parameters: %w", err)
	}
	var result entity.PaymentParameters
	if err := json.Unmarshal(decoded, &result); err != nil {
		return nil, fmt.Errorf("parse parameters: %w", err)
	}
	return &result, nil
}

// StartPaymentProcessor launches a background goroutine that periodically checks for
// unbilled transactions and processes payment retries.
func (c *Core) StartPaymentProcessor() {
	c.stopPaymentProcessor = make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				c.processUnbilledTransactions(ctx)
				c.processPaymentRetries(ctx)
				cancel()
			case <-c.stopPaymentProcessor:
				return
			}
		}
	}()
	c.log.Info("payment processor started")
}

// StopPaymentProcessor signals the payment processor goroutine to stop.
func (c *Core) StopPaymentProcessor() {
	if c.stopPaymentProcessor != nil {
		close(c.stopPaymentProcessor)
		c.log.Info("payment processor stopped")
	}
}

// processUnbilledTransactions finds all unbilled transactions and initiates payment for each.
func (c *Core) processUnbilledTransactions(ctx context.Context) {
	if c.redsys == nil {
		return
	}

	transactions, err := c.repo.GetUnbilledTransactions(ctx)
	if err != nil {
		c.log.With(sl.Err(err)).Error("failed to get unbilled transactions")
		return
	}

	for _, tx := range transactions {
		// Skip transactions that already have a retry record (handled by processPaymentRetries)
		retry, _ := c.repo.GetPaymentRetry(ctx, tx.TransactionId)
		if retry != nil {
			continue
		}

		c.log.With(slog.Int("transaction_id", tx.TransactionId)).Info("processing unbilled transaction")
		if e := c.PayTransaction(ctx, tx.TransactionId); e != nil {
			c.log.With(
				slog.Int("transaction_id", tx.TransactionId),
				sl.Err(e),
			).Warn("failed to process unbilled transaction")
		}
	}
}

// processPaymentRetries finds pending retry records and re-attempts payment.
func (c *Core) processPaymentRetries(ctx context.Context) {
	if c.redsys == nil {
		return
	}

	retries, err := c.repo.GetPendingRetries(ctx, time.Now())
	if err != nil {
		c.log.With(sl.Err(err)).Error("failed to get pending retries")
		return
	}

	for _, retry := range retries {
		log := c.log.With(
			slog.Int("transaction_id", retry.TransactionId),
			slog.Int("attempt", retry.Attempt),
		)

		transaction, err := c.repo.GetTransaction(ctx, retry.TransactionId)
		if err != nil {
			log.With(sl.Err(err)).Error("failed to get transaction for retry")
			continue
		}
		if transaction == nil {
			log.Warn("transaction not found for retry, removing retry record")
			_ = c.repo.DeletePaymentRetry(ctx, retry.TransactionId)
			continue
		}

		// Reset PaymentBilled and PaymentError so PayTransaction can proceed
		transaction.PaymentBilled = 0
		transaction.PaymentError = ""
		if e := c.repo.UpdateTransactionPayment(ctx, transaction); e != nil {
			log.With(sl.Err(e)).Error("failed to reset transaction for retry")
			continue
		}

		log.Info("retrying payment")
		if e := c.PayTransaction(ctx, retry.TransactionId); e != nil {
			log.With(sl.Err(e)).Warn("payment retry failed")
		}
	}
}

// schedulePaymentRetry creates or updates a retry record for a failed payment.
func (c *Core) schedulePaymentRetry(ctx context.Context, transactionId int, lastError string) {
	log := c.log.With(slog.Int("transaction_id", transactionId))

	existing, _ := c.repo.GetPaymentRetry(ctx, transactionId)

	var attempt int
	if existing != nil {
		attempt = existing.Attempt + 1
	} else {
		attempt = 1
	}

	if attempt > maxRetryAttempts {
		log.With(slog.Int("attempts", attempt-1)).Warn("payment retries exhausted")
		_ = c.repo.DeletePaymentRetry(ctx, transactionId)
		return
	}

	now := time.Now()
	retry := &entity.PaymentRetry{
		TransactionId: transactionId,
		Attempt:       attempt,
		NextRetryTime: now.Add(retryDelays[attempt-1]),
		LastError:     lastError,
		UpdatedAt:     now,
	}
	if existing == nil {
		retry.CreatedAt = now
	} else {
		retry.CreatedAt = existing.CreatedAt
	}

	if e := c.repo.SavePaymentRetry(ctx, retry); e != nil {
		log.With(sl.Err(e)).Error("failed to save payment retry")
		return
	}

	log.With(
		slog.Int("attempt", attempt),
		slog.Time("next_retry", retry.NextRetryTime),
	).Info("payment retry scheduled")
}
