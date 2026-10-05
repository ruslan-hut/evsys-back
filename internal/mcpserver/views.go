package mcpserver

import (
	"evsys-back/entity"
	"time"
)

// The views below are what the tools return instead of the entities: flatter,
// with units in the field names, unset times left out rather than rendered as
// year 1, and nothing that works as a credential (API tokens, card tokens).

type connectorView struct {
	ConnectorId       int                    `json:"connector_id"`
	Name              string                 `json:"name,omitempty"`
	Type              string                 `json:"type,omitempty"`
	Power             int                    `json:"power,omitempty"`
	Status            string                 `json:"status"`
	StatusTime        string                 `json:"status_time,omitempty"`
	ErrorCode         string                 `json:"error_code,omitempty"`
	Info              string                 `json:"info,omitempty"`
	VendorId          string                 `json:"vendor_id,omitempty"`
	TransactionId     int                    `json:"current_transaction_id,omitempty"`
	CurrentLimitAmps  int                    `json:"current_limit_a,omitempty"`
	LastProfileAnswer *entity.ProfileVerdict `json:"last_profile,omitempty"`
}

type chargePointView struct {
	Id              string          `json:"charge_point_id"`
	Title           string          `json:"title,omitempty"`
	Address         string          `json:"address,omitempty"`
	LocationId      string          `json:"location_id,omitempty"`
	IsEnabled       bool            `json:"is_enabled"`
	IsOnline        bool            `json:"is_online"`
	Status          string          `json:"status,omitempty"`
	StatusTime      string          `json:"status_time,omitempty"`
	LastEvent       string          `json:"last_event,omitempty"`
	ErrorCode       string          `json:"error_code,omitempty"`
	Info            string          `json:"info,omitempty"`
	Model           string          `json:"model,omitempty"`
	Vendor          string          `json:"vendor,omitempty"`
	SerialNumber    string          `json:"serial_number,omitempty"`
	FirmwareVersion string          `json:"firmware_version,omitempty"`
	SmartCharging   bool            `json:"smart_charging"`
	TriggerMessage  *bool           `json:"trigger_message,omitempty"`
	AccessType      string          `json:"access_type,omitempty"`
	AccessLevel     int             `json:"access_level"`
	Connectors      []connectorView `json:"connectors"`
}

func connectorFrom(c *entity.Connector) connectorView {
	return connectorView{
		ConnectorId:       c.Id,
		Name:              c.IdName,
		Type:              c.Type,
		Power:             c.Power,
		Status:            c.Status,
		StatusTime:        timestamp(c.StatusTime),
		ErrorCode:         c.ErrorCode,
		Info:              c.Info,
		VendorId:          c.VendorId,
		TransactionId:     c.TransactionId,
		CurrentLimitAmps:  c.CurrentPowerLimit,
		LastProfileAnswer: c.LastProfile,
	}
}

func chargePointFrom(cp *entity.ChargePoint) chargePointView {
	view := chargePointView{
		Id:              cp.Id,
		Title:           cp.Title,
		Address:         cp.Address,
		LocationId:      cp.LocationId,
		IsEnabled:       cp.IsEnabled,
		IsOnline:        cp.IsOnline,
		Status:          cp.Status,
		StatusTime:      timestamp(cp.StatusTime),
		LastEvent:       timestamp(cp.EventTime),
		ErrorCode:       cp.ErrorCode,
		Info:            cp.Info,
		Model:           cp.Model,
		Vendor:          cp.Vendor,
		SerialNumber:    cp.SerialNumber,
		FirmwareVersion: cp.FirmwareVersion,
		SmartCharging:   cp.SmartCharging,
		TriggerMessage:  cp.TriggerMessage,
		AccessType:      cp.AccessType,
		AccessLevel:     cp.AccessLevel,
		Connectors:      make([]connectorView, 0, len(cp.Connectors)),
	}
	for _, c := range cp.Connectors {
		if c != nil {
			view.Connectors = append(view.Connectors, connectorFrom(c))
		}
	}
	return view
}

// hasError reports whether an OCPP error code describes an actual error.
func hasError(code string) bool {
	return code != "" && code != "NoError"
}

// meterView is one meter reading of a session.
type meterView struct {
	Time            string  `json:"time"`
	ConsumedWh      int     `json:"consumed_wh"`
	PowerW          int     `json:"power_w"`
	PowerActiveW    int     `json:"power_active_w,omitempty"`
	VoltageV        float64 `json:"voltage_v,omitempty"`
	CurrentImportA  float64 `json:"current_import_a,omitempty"`
	CurrentOfferedA float64 `json:"current_offered_a,omitempty"`
	BatteryLevel    int     `json:"battery_level,omitempty"`
	Price           int     `json:"price,omitempty"`
	ConnectorStatus string  `json:"connector_status,omitempty"`
}

func meterFrom(m *entity.TransactionMeter) meterView {
	return meterView{
		Time:            timestamp(m.Time),
		ConsumedWh:      m.ConsumedEnergy,
		PowerW:          m.PowerRate,
		PowerActiveW:    m.PowerActive,
		VoltageV:        m.Voltage,
		CurrentImportA:  m.CurrentImport,
		CurrentOfferedA: m.CurrentOffered,
		BatteryLevel:    m.BatteryLevel,
		Price:           m.Price,
		ConnectorStatus: m.ConnectorStatus,
	}
}

func metersFrom(values []entity.TransactionMeter) []meterView {
	views := make([]meterView, 0, len(values))
	for i := range values {
		views = append(views, meterFrom(&values[i]))
	}
	return views
}

// transactionRow summarizes a finished transaction.
type transactionRow struct {
	TransactionId   int     `json:"transaction_id"`
	ChargePointId   string  `json:"charge_point_id"`
	ConnectorId     int     `json:"connector_id"`
	Username        string  `json:"username,omitempty"`
	IdTag           string  `json:"id_tag"`
	IdTagNote       string  `json:"id_tag_note,omitempty"`
	TimeStart       string  `json:"time_start"`
	TimeStop        string  `json:"time_stop,omitempty"`
	DurationMinutes float64 `json:"duration_min"`
	EnergyWh        int     `json:"energy_wh"`
	AveragePowerW   int     `json:"avg_power_w"`
	PowerLimitAmps  int     `json:"power_limit_a,omitempty"`
	Reason          string  `json:"reason,omitempty"`
	PaymentAmount   int     `json:"payment_amount"`
	PaymentBilled   int     `json:"payment_billed"`
	PaymentError    string  `json:"payment_error,omitempty"`
}

func transactionEnergy(t *entity.Transaction) int {
	if t.MeterStop > t.MeterStart {
		return t.MeterStop - t.MeterStart
	}
	return 0
}

func transactionRowFrom(t *entity.Transaction) transactionRow {
	energy := transactionEnergy(t)
	duration := t.TimeStop.Sub(t.TimeStart)
	avg := 0
	if duration > 0 {
		avg = int(float64(energy) / duration.Hours())
	}
	username, note := t.Username, t.IdTagNote
	if t.UserTag != nil {
		if username == "" {
			username = t.UserTag.Username
		}
		if note == "" {
			note = t.UserTag.Note
		}
	}
	return transactionRow{
		TransactionId:   t.TransactionId,
		ChargePointId:   t.ChargePointId,
		ConnectorId:     t.ConnectorId,
		Username:        username,
		IdTag:           t.IdTag,
		IdTagNote:       note,
		TimeStart:       timestamp(t.TimeStart),
		TimeStop:        timestamp(t.TimeStop),
		DurationMinutes: minutes(duration),
		EnergyWh:        energy,
		AveragePowerW:   avg,
		PowerLimitAmps:  t.PowerLimit,
		Reason:          t.Reason,
		PaymentAmount:   t.PaymentAmount,
		PaymentBilled:   t.PaymentBilled,
		PaymentError:    t.PaymentError,
	}
}

// activeSessionView summarizes a running transaction.
type activeSessionView struct {
	TransactionId    int        `json:"transaction_id"`
	ChargePointId    string     `json:"charge_point_id"`
	ChargePointTitle string     `json:"charge_point_title,omitempty"`
	ConnectorId      int        `json:"connector_id"`
	ConnectorStatus  string     `json:"connector_status,omitempty"`
	Username         string     `json:"username,omitempty"`
	IdTag            string     `json:"id_tag"`
	IdTagNote        string     `json:"id_tag_note,omitempty"`
	TimeStart        string     `json:"time_start"`
	DurationMinutes  float64    `json:"duration_min"`
	ConsumedWh       int        `json:"consumed_wh"`
	PowerW           int        `json:"power_w"`
	PowerLimitAmps   int        `json:"power_limit_a,omitempty"`
	Price            int        `json:"price"`
	LastMeter        *meterView `json:"last_meter,omitempty"`
}

func activeSessionFrom(s *entity.ChargeState, now time.Time) activeSessionView {
	view := activeSessionView{
		TransactionId:    s.TransactionId,
		ChargePointId:    s.ChargePointId,
		ChargePointTitle: s.ChargePointTitle,
		ConnectorId:      s.ConnectorId,
		ConnectorStatus:  s.Status,
		Username:         s.Username,
		IdTag:            s.IdTag,
		IdTagNote:        s.IdTagNote,
		TimeStart:        timestamp(s.TimeStarted),
		DurationMinutes:  minutes(now.Sub(s.TimeStarted)),
		ConsumedWh:       s.Consumed,
		PowerW:           s.PowerRate,
		PowerLimitAmps:   s.PowerLimit,
		Price:            s.Price,
	}
	if s.UserTag != nil {
		if view.Username == "" {
			view.Username = s.UserTag.Username
		}
		if view.IdTagNote == "" {
			view.IdTagNote = s.UserTag.Note
		}
	}
	if n := len(s.MeterValues); n > 0 {
		last := meterFrom(&s.MeterValues[n-1])
		view.LastMeter = &last
	}
	return view
}

// paymentMethodView is a payment method without its card token and network
// transaction id, which together allow charging the card.
type paymentMethodView struct {
	Description string `json:"description,omitempty"`
	CardNumber  string `json:"card_number,omitempty"`
	CardBrand   string `json:"card_brand,omitempty"`
	CardCountry string `json:"card_country,omitempty"`
	ExpiryDate  string `json:"expiry_date,omitempty"`
	IsDefault   bool   `json:"is_default"`
	UserName    string `json:"user_name,omitempty"`
	FailCount   int    `json:"fail_count"`
}

func paymentMethodFrom(pm *entity.PaymentMethod) *paymentMethodView {
	if pm == nil {
		return nil
	}
	return &paymentMethodView{
		Description: pm.Description,
		CardNumber:  pm.CardNumber,
		CardBrand:   pm.CardBrand,
		CardCountry: pm.CardCountry,
		ExpiryDate:  pm.ExpiryDate,
		IsDefault:   pm.IsDefault,
		UserName:    pm.UserName,
		FailCount:   pm.FailCount,
	}
}

// paymentOrderView is a payment order without the card token it was charged
// to.
type paymentOrderView struct {
	Order        int    `json:"order"`
	Amount       int    `json:"amount"`
	Currency     string `json:"currency,omitempty"`
	Description  string `json:"description,omitempty"`
	IsCompleted  bool   `json:"is_completed"`
	Result       string `json:"result,omitempty"`
	TimeOpened   string `json:"time_opened,omitempty"`
	TimeClosed   string `json:"time_closed,omitempty"`
	RefundAmount int    `json:"refund_amount,omitempty"`
	RefundTime   string `json:"refund_time,omitempty"`
}

func paymentOrdersFrom(orders []entity.PaymentOrder) []paymentOrderView {
	views := make([]paymentOrderView, 0, len(orders))
	for _, o := range orders {
		views = append(views, paymentOrderView{
			Order:        o.Order,
			Amount:       o.Amount,
			Currency:     o.Currency,
			Description:  o.Description,
			IsCompleted:  o.IsCompleted,
			Result:       o.Result,
			TimeOpened:   timestamp(o.TimeOpened),
			TimeClosed:   timestamp(o.TimeClosed),
			RefundAmount: o.RefundAmount,
			RefundTime:   timestamp(o.RefundTime),
		})
	}
	return views
}

// transactionDetail is a full transaction state. The outer fields shadow the
// embedded ones of the same JSON name.
type transactionDetail struct {
	*entity.ChargeState
	TimeStart       string             `json:"time_started"`
	TimeStop        string             `json:"time_stop,omitempty"`
	DurationMinutes float64            `json:"duration_min"`
	PaymentMethod   *paymentMethodView `json:"payment_method,omitempty"`
	PaymentOrders   []paymentOrderView `json:"payment_orders,omitempty"`
	MeterValueCount int                `json:"meter_value_count"`
	MeterValues     []meterView        `json:"meter_values"`
	// CanStop and Duration only make sense to the user who started the session
	// in the app.
	CanStop  *bool `json:"can_stop,omitempty"`
	Duration *int  `json:"duration,omitempty"`
}

type userView struct {
	Username       string `json:"username"`
	Name           string `json:"name,omitempty"`
	Email          string `json:"email,omitempty"`
	Role           string `json:"role,omitempty"`
	AccessLevel    int    `json:"access_level"`
	Group          string `json:"group,omitempty"`
	PaymentPlan    string `json:"payment_plan,omitempty"`
	DateRegistered string `json:"date_registered,omitempty"`
	LastSeen       string `json:"last_seen,omitempty"`
}

func userFromEntity(u *entity.User) userView {
	return userView{
		Username:       u.Username,
		Name:           u.Name,
		Email:          u.Email,
		Role:           u.Role,
		AccessLevel:    u.AccessLevel,
		Group:          u.Group,
		PaymentPlan:    u.PaymentPlan,
		DateRegistered: timestamp(u.DateRegistered),
		LastSeen:       timestamp(u.LastSeen),
	}
}

type userTagView struct {
	IdTag          string `json:"id_tag"`
	Username       string `json:"username"`
	Note           string `json:"note,omitempty"`
	Source         string `json:"source,omitempty"`
	IsEnabled      bool   `json:"is_enabled"`
	Local          bool   `json:"local"`
	DateRegistered string `json:"date_registered,omitempty"`
	LastSeen       string `json:"last_seen,omitempty"`
}

func userTagFrom(t *entity.UserTag) userTagView {
	return userTagView{
		IdTag:          t.IdTag,
		Username:       t.Username,
		Note:           t.Note,
		Source:         t.Source,
		IsEnabled:      t.IsEnabled,
		Local:          t.Local,
		DateRegistered: timestamp(t.DateRegistered),
		LastSeen:       timestamp(t.LastSeen),
	}
}
