package core

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"time"
)

const (
	MaxAccessLevel              int = 10
	NormalizedMeterValuesLength     = 60
	subSystemReports                = "reports"
)

type Core struct {
	repo     Repository
	auth     Authenticator
	cs       CentralSystem
	reports  Reports
	redsys   RedsysClient
	currency string
	log      *slog.Logger
}

// RedsysClient interface for Redsys operations
type RedsysClient interface {
	Capture(ctx context.Context, req CaptureRequest) (*CaptureResponse, error)
	Cancel(ctx context.Context, req CaptureRequest) (*CaptureResponse, error)
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

func (c *Core) GetConfig(ctx context.Context, name string) (interface{}, error) {
	return c.repo.GetConfig(ctx, name)
}

func (c *Core) GetLog(ctx context.Context, name string) (interface{}, error) {
	return c.repo.ReadLog(ctx, name)
}

func (c *Core) AuthenticateByToken(ctx context.Context, token string) (*entity.User, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	user, err := c.auth.AuthenticateByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if user != nil {
		user.Password = ""
	}
	return user, nil
}

func (c *Core) AuthenticateUser(ctx context.Context, username, password string) (*entity.User, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	user, err := c.auth.AuthenticateUser(ctx, username, password)
	if err != nil {
		return nil, err
	}
	if user != nil {
		user.Password = ""
	}
	return user, nil
}

func (c *Core) AddUser(ctx context.Context, user *entity.User) (*entity.User, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
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
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	return c.auth.GetUsers(ctx, user)
}

func (c *Core) CreateUser(ctx context.Context, author *entity.User, user *entity.User) (*entity.User, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	if !author.IsPowerUser() {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}
	err := c.auth.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	user.Password = ""
	return user, nil
}

func (c *Core) UpdateUser(ctx context.Context, author *entity.User, username string, updates *entity.UserUpdate) (*entity.User, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	if !author.IsPowerUser() {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}
	updated, err := c.auth.UpdateUser(ctx, username, updates)
	if err != nil {
		return nil, err
	}
	if updated != nil {
		updated.Password = ""
	}
	return updated, nil
}

func (c *Core) DeleteUser(ctx context.Context, author *entity.User, username string) error {
	if c.auth == nil {
		return fmt.Errorf("authenticator not set")
	}
	if !author.IsPowerUser() {
		return fmt.Errorf("access denied: insufficient permissions")
	}
	return c.auth.DeleteUser(ctx, username)
}

func (c *Core) UserTag(ctx context.Context, user *entity.User) (string, error) {
	if c.auth == nil {
		return "", fmt.Errorf("authenticator not set")
	}
	if user == nil {
		return "", fmt.Errorf("user is nil")
	}
	return c.auth.GetUserTag(ctx, user)
}

func (c *Core) ListUserTags(ctx context.Context, author *entity.User) ([]*entity.UserTag, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	if !author.IsPowerUser() {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}
	return c.auth.ListUserTags(ctx)
}

func (c *Core) GetUserTag(ctx context.Context, author *entity.User, idTag string) (*entity.UserTag, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	if !author.IsPowerUser() {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}
	return c.auth.GetUserTagByIdTag(ctx, idTag)
}

func (c *Core) CreateUserTag(ctx context.Context, author *entity.User, tag *entity.UserTagCreate) (*entity.UserTag, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	if !author.IsPowerUser() {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}
	return c.auth.CreateUserTag(ctx, tag)
}

func (c *Core) UpdateUserTag(ctx context.Context, author *entity.User, idTag string, updates *entity.UserTagUpdate) (*entity.UserTag, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("authenticator not set")
	}
	if !author.IsPowerUser() {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}
	return c.auth.UpdateUserTag(ctx, idTag, updates)
}

func (c *Core) DeleteUserTag(ctx context.Context, author *entity.User, idTag string) error {
	if c.auth == nil {
		return fmt.Errorf("authenticator not set")
	}
	if !author.IsPowerUser() {
		return fmt.Errorf("access denied: insufficient permissions")
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

// CreatePreauthorizationOrder creates a new preauthorization order
func (c *Core) CreatePreauthorizationOrder(ctx context.Context, user *entity.User, req *entity.PreauthorizationOrderRequest) (*entity.PreauthorizationOrderResponse, error) {
	if user == nil {
		return nil, fmt.Errorf("user is nil")
	}
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// Generate order number based on last order
	lastOrder, _ := c.repo.GetLastPreauthorizationOrder(ctx)
	var orderNum int
	if lastOrder != nil {
		// Parse last order number and increment
		fmt.Sscanf(lastOrder.OrderNumber, "%d", &orderNum)
		orderNum++
	} else {
		orderNum = 1200 // Start from 1200
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

	err := c.repo.SavePreauthorization(ctx, preauth)
	if err != nil {
		return nil, fmt.Errorf("failed to save preauthorization: %w", err)
	}

	return &entity.PreauthorizationOrderResponse{
		Order:           orderNum,
		Amount:          req.Amount,
		Description:     req.Description,
		TransactionType: transactionType,
		PaymentMethodId: req.PaymentMethodId,
		TransactionId:   req.TransactionId,
	}, nil
}

// SavePreauthorization saves a preauthorization result from Redsys callback
func (c *Core) SavePreauthorization(ctx context.Context, user *entity.User, req *entity.PreauthorizationSaveRequest) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	preauth, err := c.repo.GetPreauthorization(ctx, req.OrderNumber)
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

	preauth, err := c.repo.GetPreauthorization(ctx, req.OriginalOrder)
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

	// Call Redsys to capture
	captureReq := CaptureRequest{
		OrderNumber:       req.OriginalOrder,
		Amount:            req.Amount,
		AuthorizationCode: authCode,
	}

	resp, err := c.redsys.Capture(ctx, captureReq)
	if err != nil {
		return nil, fmt.Errorf("failed to capture: %w", err)
	}

	// Parse order number for response
	var orderNum int
	fmt.Sscanf(req.OriginalOrder, "%d", &orderNum)

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

	preauth, err := c.repo.GetPreauthorization(ctx, req.OrderNumber)
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
