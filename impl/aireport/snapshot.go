package aireport

import (
	"context"
	"encoding/json"
	"evsys-back/entity"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	// allAccess reads every charge point regardless of its access level.
	allAccess = 10
	// maxSnapshotBytes bounds the JSON the model reads; lists are cut until
	// the snapshot fits, and the cut is noted in it.
	maxSnapshotBytes = 160_000
	maxLogRecords    = 50_000

	maxSysEvents      = 80
	maxMessageEvents  = 40
	maxErrorRows      = 40
	maxProblemRows    = 60
	maxRetryRows      = 20
	maxDeliveryRows   = 10
	shortSessionLimit = 2 * time.Minute
)

// Snapshot is everything the model reads about one period. It holds charge
// point ids, counts and log shapes - no user names, tags, emails or cards.
type Snapshot struct {
	GeneratedAt     time.Time        `json:"generated_at"`
	TimeZone        string           `json:"time_zone"`
	Period          Period           `json:"period"`
	PreviousPeriod  Period           `json:"previous_period"`
	Network         *NetworkState    `json:"network"`
	Stations        []*StationWeek   `json:"stations"`
	ConnectorErrors []*ErrorRow      `json:"connector_errors"`
	SysLog          *LogDigest       `json:"sys_log"`
	BackendLog      *LogDigest       `json:"backend_log"`
	PaymentLog      *LogDigest       `json:"payment_log"`
	Sessions        *SessionsSummary `json:"sessions"`
	Payments        *PaymentsState   `json:"payments"`
	Webhooks        *WebhooksState   `json:"webhooks"`
	// Notes tell the model what is missing or cut, so it does not read an
	// absence as good news.
	Notes []string `json:"notes,omitempty"`
}

type Period struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// NetworkState is the state at the moment of the run.
type NetworkState struct {
	ChargePoints      int            `json:"charge_points"`
	Enabled           int            `json:"enabled"`
	Online            int            `json:"online"`
	Offline           []*OfflineRow  `json:"offline"`
	ConnectorStatuses map[string]int `json:"connector_statuses"`
	Problems          []*ProblemRow  `json:"problems"`
}

type OfflineRow struct {
	ChargePointId string    `json:"charge_point_id"`
	Title         string    `json:"title,omitempty"`
	LastEvent     time.Time `json:"last_event"`
}

// ProblemRow is a charge point or connector reporting a non-working status
// now; connector 0 is the charge point itself.
type ProblemRow struct {
	ChargePointId string    `json:"charge_point_id"`
	ConnectorId   int       `json:"connector_id"`
	Status        string    `json:"status"`
	ErrorCode     string    `json:"error_code,omitempty"`
	Info          string    `json:"info,omitempty"`
	Since         time.Time `json:"since"`
	Online        bool      `json:"charge_point_online"`
}

// StationWeek sums the period for one enabled charge point, with the previous
// period's figures for comparison.
type StationWeek struct {
	ChargePointId string `json:"charge_point_id"`
	Model         string `json:"model,omitempty"`
	Vendor        string `json:"vendor,omitempty"`
	Firmware      string `json:"firmware,omitempty"`
	// Uptime is absent when the log has no connection events of the station.
	UptimePercent      *float64 `json:"uptime_percent,omitempty"`
	PrevUptimePercent  *float64 `json:"prev_uptime_percent,omitempty"`
	Disconnects        int      `json:"disconnects"`
	PrevDisconnects    int      `json:"prev_disconnects"`
	KeepaliveTimeouts  int      `json:"keepalive_timeouts"`
	ReadErrors         int      `json:"read_errors"`
	Boots              int      `json:"boots"`
	PrevBoots          int      `json:"prev_boots"`
	FaultedReports     int      `json:"faulted_reports"`
	PrevFaultedReports int      `json:"prev_faulted_reports"`
	UnavailableReports int      `json:"unavailable_reports"`
	ConnectorErrors    int      `json:"connector_errors"`
	PrevConnectorErrs  int      `json:"prev_connector_errors"`
	Sessions           int      `json:"sessions"`
	EnergyKWh          float64  `json:"energy_kwh"`
}

type ErrorRow struct {
	ChargePointId   string    `json:"charge_point_id"`
	ConnectorId     int       `json:"connector_id"`
	ErrorCode       string    `json:"error_code"`
	VendorErrorCode string    `json:"vendor_error_code,omitempty"`
	Count           int       `json:"count"`
	First           time.Time `json:"first"`
	Last            time.Time `json:"last"`
	LastInfo        string    `json:"last_info,omitempty"`
}

// LogDigest counts the records of one log in the period and lists the shapes
// of the notable ones.
type LogDigest struct {
	Records    int            `json:"records"`
	Truncated  bool           `json:"truncated,omitempty"`
	ByCategory map[string]int `json:"by_category"`
	ByLevel    map[string]int `json:"by_level,omitempty"`
	Events     []*LogEvent    `json:"events"`
	// OmittedEvents counts notable shapes left out of Events for size.
	OmittedEvents int `json:"omitted_events,omitempty"`
}

// LogEvent is one shape of record: the text with numbers, addresses and ids
// replaced, how often it occurred and when.
type LogEvent struct {
	ChargePointId string    `json:"charge_point_id,omitempty"`
	Category      string    `json:"category"`
	Level         string    `json:"level,omitempty"`
	Template      string    `json:"template"`
	Count         int       `json:"count"`
	First         time.Time `json:"first"`
	Last          time.Time `json:"last"`
}

type SessionsSummary struct {
	Finished        int            `json:"finished"`
	PrevFinished    int            `json:"prev_finished"`
	EnergyKWh       float64        `json:"energy_kwh"`
	PrevEnergyKWh   float64        `json:"prev_energy_kwh"`
	BilledEUR       float64        `json:"billed_eur"`
	PrevBilledEUR   float64        `json:"prev_billed_eur"`
	ZeroEnergy      int            `json:"zero_energy"`
	Short           int            `json:"shorter_than_2_min"`
	PaymentErrors   int            `json:"payment_errors"`
	StopReasons     map[string]int `json:"stop_reasons"`
	ActiveNow       int            `json:"active_now"`
	ByChargePoint   []*SessionsRow `json:"by_charge_point"`
	PaymentErrorsBy map[string]int `json:"payment_errors_by_message,omitempty"`
}

type SessionsRow struct {
	ChargePointId string  `json:"charge_point_id"`
	Sessions      int     `json:"sessions"`
	EnergyKWh     float64 `json:"energy_kwh"`
	ZeroEnergy    int     `json:"zero_energy"`
	Short         int     `json:"shorter_than_2_min"`
	PaymentErrors int     `json:"payment_errors"`
}

type PaymentsState struct {
	RetryQueue int         `json:"retry_queue"`
	Retries    []*RetryRow `json:"retries"`
}

type RetryRow struct {
	TransactionId int       `json:"transaction_id"`
	Attempt       int       `json:"attempt"`
	NextRetry     time.Time `json:"next_retry"`
	LastError     string    `json:"last_error"`
}

type WebhooksState struct {
	Subscribers []*entity.WebhookOutboxStats `json:"subscribers"`
	Problems    int                          `json:"problem_deliveries"`
	Recent      []*DeliveryRow               `json:"recent_problems"`
}

type DeliveryRow struct {
	Subscriber string    `json:"subscriber"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	Attempts   int       `json:"attempts"`
	LastError  string    `json:"last_error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// collect reads the snapshot of [from, to) with [prevFrom, from) for
// comparison. A source that fails is noted and skipped, so one broken
// aggregation does not cost the whole report.
func (s *Service) collect(ctx context.Context, from, to, prevFrom time.Time, tz string) *Snapshot {
	snap := &Snapshot{
		GeneratedAt:    s.now().UTC(),
		TimeZone:       tz,
		Period:         Period{From: from.UTC(), To: to.UTC()},
		PreviousPeriod: Period{From: prevFrom.UTC(), To: from.UTC()},
	}
	note := func(source string, err error) {
		snap.Notes = append(snap.Notes, fmt.Sprintf("%s unavailable: %v", source, err))
	}
	// the period is [from, to); the stores filter inclusively
	toIncl := to.Add(-time.Millisecond)
	fromIncl := from.Add(-time.Millisecond)

	stations := make(map[string]*StationWeek)
	station := func(id string) *StationWeek {
		st, ok := stations[id]
		if !ok {
			st = &StationWeek{ChargePointId: id}
			stations[id] = st
		}
		return st
	}

	chargePoints, err := s.repo.GetChargePoints(ctx, allAccess, "")
	if err != nil {
		note("charge points", err)
	}
	enabled := make(map[string]bool)
	snap.Network = networkState(chargePoints)
	for _, cp := range chargePoints {
		if !cp.IsEnabled {
			continue
		}
		enabled[cp.Id] = true
		st := station(cp.Id)
		st.Model, st.Vendor, st.Firmware = cp.Model, cp.Vendor, cp.FirmwareVersion
	}

	if rows, err := s.repo.StationUptime(ctx, from, to, ""); err != nil {
		note("uptime", err)
	} else {
		for _, r := range rows {
			v := round1(r.UptimePercent)
			station(r.ChargePointId).UptimePercent = &v
		}
	}
	if rows, err := s.repo.StationUptime(ctx, prevFrom, from, ""); err != nil {
		note("previous uptime", err)
	} else {
		for _, r := range rows {
			v := round1(r.UptimePercent)
			station(r.ChargePointId).PrevUptimePercent = &v
		}
	}

	if rows, err := s.repo.ErrorSummary(ctx, from, toIncl, ""); err != nil {
		note("connector errors", err)
	} else {
		sort.Slice(rows, func(i, j int) bool { return rows[i].Count > rows[j].Count })
		for _, r := range rows {
			station(r.ChargePointId).ConnectorErrors += r.Count
			if len(snap.ConnectorErrors) < maxErrorRows {
				snap.ConnectorErrors = append(snap.ConnectorErrors, &ErrorRow{
					ChargePointId:   r.ChargePointId,
					ConnectorId:     r.ConnectorId,
					ErrorCode:       r.ErrorCode,
					VendorErrorCode: r.VendorErrorCode,
					Count:           r.Count,
					First:           r.First,
					Last:            r.Last,
					LastInfo:        templateOf(r.LastInfo),
				})
			}
		}
		if len(rows) > maxErrorRows {
			snap.Notes = append(snap.Notes, fmt.Sprintf("connector_errors lists the %d most frequent of %d rows", maxErrorRows, len(rows)))
		}
	}
	if rows, err := s.repo.ErrorSummary(ctx, prevFrom, fromIncl, ""); err != nil {
		note("previous connector errors", err)
	} else {
		for _, r := range rows {
			station(r.ChargePointId).PrevConnectorErrs += r.Count
		}
	}

	sysLog, err := s.readSysLog(ctx, from, toIncl)
	if err != nil {
		note("sys_log", err)
	} else {
		snap.SysLog = digestSysLog(sysLog, station)
		if len(sysLog) >= maxLogRecords {
			snap.SysLog.Truncated = true
			snap.Notes = append(snap.Notes, fmt.Sprintf("sys_log holds more than %d records in the period: only the newest were read, so station counters miss the start of the period", maxLogRecords))
		}
	}
	if prev, err := s.readSysLog(ctx, prevFrom, fromIncl); err != nil {
		note("previous sys_log", err)
	} else {
		if len(prev) >= maxLogRecords {
			snap.Notes = append(snap.Notes, "previous sys_log was cut at the record limit: prev_* station counters are partial")
		}
		for _, r := range prev {
			c := classify(r)
			st := station(r.ChargePointId)
			if c.disconnect {
				st.PrevDisconnects++
			}
			if c.boot {
				st.PrevBoots++
			}
			if c.faulted {
				st.PrevFaultedReports++
			}
		}
	}

	if digest, err := s.readMessageLog(ctx, "back", from, toIncl); err != nil {
		note("back_log", err)
	} else {
		snap.BackendLog = digest
		if digest.Truncated {
			snap.Notes = append(snap.Notes, "back_log was cut at the record limit: only the newest records were read")
		}
	}
	if digest, err := s.readMessageLog(ctx, "pay", from, toIncl); err != nil {
		note("payment_log", err)
	} else {
		snap.PaymentLog = digest
		if digest.Truncated {
			snap.Notes = append(snap.Notes, "payment_log was cut at the record limit: only the newest records were read")
		}
	}

	snap.Sessions = s.sessions(ctx, from, toIncl, prevFrom, fromIncl, station, note)
	snap.Sessions.ActiveNow = activeSessions(chargePoints)
	snap.Payments = s.payments(ctx, note)
	snap.Webhooks = s.webhooks(ctx, note)

	// '*' marks central-system records that belong to no charge point
	delete(stations, "*")
	delete(stations, "")
	for id, st := range stations {
		if len(enabled) > 0 && !enabled[id] && st.Sessions == 0 && st.ConnectorErrors == 0 {
			continue
		}
		snap.Stations = append(snap.Stations, st)
	}
	sort.Slice(snap.Stations, func(i, j int) bool {
		return snap.Stations[i].ChargePointId < snap.Stations[j].ChargePointId
	})
	return snap
}

func networkState(chargePoints []*entity.ChargePoint) *NetworkState {
	ns := &NetworkState{
		ChargePoints:      len(chargePoints),
		ConnectorStatuses: make(map[string]int),
	}
	for _, cp := range chargePoints {
		if !cp.IsEnabled {
			continue
		}
		ns.Enabled++
		if cp.IsOnline {
			ns.Online++
		} else {
			ns.Offline = append(ns.Offline, &OfflineRow{ChargePointId: cp.Id, Title: cp.Title, LastEvent: cp.EventTime})
		}
		if problemStatus(cp.Status) || cp.ErrorCode != "" && cp.ErrorCode != "NoError" {
			ns.addProblem(&ProblemRow{
				ChargePointId: cp.Id,
				Status:        cp.Status,
				ErrorCode:     cp.ErrorCode,
				Info:          templateOf(cp.Info),
				Since:         cp.StatusTime,
				Online:        cp.IsOnline,
			})
		}
		for _, c := range cp.Connectors {
			ns.ConnectorStatuses[c.Status]++
			if problemStatus(c.Status) || c.ErrorCode != "" && c.ErrorCode != "NoError" {
				ns.addProblem(&ProblemRow{
					ChargePointId: cp.Id,
					ConnectorId:   c.Id,
					Status:        c.Status,
					ErrorCode:     c.ErrorCode,
					Info:          templateOf(c.Info),
					Since:         c.StatusTime,
					Online:        cp.IsOnline,
				})
			}
		}
	}
	return ns
}

func (ns *NetworkState) addProblem(p *ProblemRow) {
	if len(ns.Problems) < maxProblemRows {
		ns.Problems = append(ns.Problems, p)
	}
}

func problemStatus(status string) bool {
	return status == "Faulted" || status == "Unavailable"
}

func activeSessions(chargePoints []*entity.ChargePoint) int {
	n := 0
	for _, cp := range chargePoints {
		for _, c := range cp.Connectors {
			if c.TransactionId > 0 {
				n++
			}
		}
	}
	return n
}

func (s *Service) readSysLog(ctx context.Context, from, to time.Time) ([]entity.FeatureMessage, error) {
	raw, err := s.repo.ReadLog(ctx, "sys", &entity.LogFilter{From: &from, To: &to, Limit: maxLogRecords})
	if err != nil || raw == nil {
		return nil, err
	}
	records, ok := raw.([]entity.FeatureMessage)
	if !ok {
		return nil, fmt.Errorf("unexpected sys log type %T", raw)
	}
	return records, nil
}

// sysClass is what a sys_log record says about its charge point.
type sysClass struct {
	disconnect, keepalive, readError, boot, faulted, unavailable bool
}

func classify(r entity.FeatureMessage) sysClass {
	text := r.Text
	var c sysClass
	switch r.Feature {
	case "WebSocket":
		c.disconnect = strings.HasPrefix(text, "unregistered")
		c.keepalive = strings.HasPrefix(text, "silent for") || strings.HasPrefix(text, "keepalive ping failed")
		c.readError = strings.HasPrefix(text, "read error")
	case "BootNotification":
		c.boot = true
	case "StatusNotification":
		c.faulted = strings.Contains(text, "Faulted")
		c.unavailable = strings.Contains(text, "Unavailable")
	}
	return c
}

// routineFeatures are the OCPP exchanges of normal operation; their records
// are counted but their shapes are listed only when they look like trouble.
var routineFeatures = map[string]bool{
	"StatusNotification":     true,
	"MeterValues":            true,
	"Heartbeat":              true,
	"CheckListenTransaction": true,
	"Trigger":                true,
	"LoadBalancer":           true,
	"DataTransfer":           true,
	"StartTransaction":       true,
	"StopTransaction":        true,
	"Authorize":              true,
	"ChangeConfiguration":    true,
	"info":                   true,
}

var troubleWords = []string{"faulted", "unavailable", "error", "fail", "reject", "refus", "timeout", "timed out", "invalid", "denied", "not found", "warning"}

func notableSys(r entity.FeatureMessage) bool {
	if !routineFeatures[r.Feature] {
		return true
	}
	if imp := strings.TrimSpace(r.Importance); imp != "" {
		return true
	}
	text := strings.ToLower(r.Text)
	for _, w := range troubleWords {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}

func digestSysLog(records []entity.FeatureMessage, station func(string) *StationWeek) *LogDigest {
	d := &LogDigest{Records: len(records), ByCategory: make(map[string]int)}
	events := newEventSet()
	for _, r := range records {
		d.ByCategory[r.Feature]++
		c := classify(r)
		if r.ChargePointId != "" && r.ChargePointId != "*" {
			st := station(r.ChargePointId)
			if c.disconnect {
				st.Disconnects++
			}
			if c.keepalive {
				st.KeepaliveTimeouts++
			}
			if c.readError {
				st.ReadErrors++
			}
			if c.boot {
				st.Boots++
			}
			if c.faulted {
				st.FaultedReports++
			}
			if c.unavailable {
				st.UnavailableReports++
			}
		}
		if notableSys(r) {
			events.add(r.ChargePointId, r.Feature, strings.TrimSpace(r.Importance), r.Text, r.Timestamp)
		}
	}
	d.Events, d.OmittedEvents = events.top(maxSysEvents)
	return d
}

// notableLevels are the levels of back and pay records whose shapes are
// listed; info records are only counted.
var notableLevels = map[string]bool{"error": true, "warn": true, "warning": true, "fatal": true, "panic": true}

func (s *Service) readMessageLog(ctx context.Context, name string, from, to time.Time) (*LogDigest, error) {
	raw, err := s.repo.ReadLog(ctx, name, &entity.LogFilter{From: &from, To: &to, Limit: maxLogRecords})
	if err != nil {
		return nil, err
	}
	records, ok := raw.([]entity.LogMessage)
	if !ok && raw != nil {
		return nil, fmt.Errorf("unexpected %s log type %T", name, raw)
	}
	d := &LogDigest{
		Records:    len(records),
		Truncated:  len(records) >= maxLogRecords,
		ByCategory: make(map[string]int),
		ByLevel:    make(map[string]int),
	}
	events := newEventSet()
	for _, r := range records {
		level := strings.ToLower(strings.TrimSpace(r.Level))
		d.ByCategory[r.Category]++
		d.ByLevel[level]++
		if notableLevels[level] {
			events.add("", r.Category, level, r.Text, r.Timestamp)
		}
	}
	d.Events, d.OmittedEvents = events.top(maxMessageEvents)
	return d, nil
}

type eventKey struct {
	chargePoint, category, level, template string
}

type eventSet struct {
	byKey map[eventKey]*LogEvent
}

func newEventSet() *eventSet {
	return &eventSet{byKey: make(map[eventKey]*LogEvent)}
}

func (e *eventSet) add(chargePoint, category, level, text string, at time.Time) {
	if chargePoint == "*" {
		chargePoint = ""
	}
	key := eventKey{chargePoint, category, level, templateOf(text)}
	ev, ok := e.byKey[key]
	if !ok {
		ev = &LogEvent{
			ChargePointId: chargePoint,
			Category:      category,
			Level:         level,
			Template:      key.template,
			First:         at,
			Last:          at,
		}
		e.byKey[key] = ev
	}
	ev.Count++
	if at.Before(ev.First) {
		ev.First = at
	}
	if at.After(ev.Last) {
		ev.Last = at
	}
}

// top returns the n most frequent shapes and how many were left out.
func (e *eventSet) top(n int) ([]*LogEvent, int) {
	list := make([]*LogEvent, 0, len(e.byKey))
	for _, ev := range e.byKey {
		list = append(list, ev)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		if list[i].ChargePointId != list[j].ChargePointId {
			return list[i].ChargePointId < list[j].ChargePointId
		}
		return list[i].Template < list[j].Template
	})
	if len(list) <= n {
		return list, 0
	}
	return list[:n], len(list) - n
}

func (s *Service) sessions(ctx context.Context, from, to, prevFrom, prevTo time.Time,
	station func(string) *StationWeek, note func(string, error)) *SessionsSummary {
	sum := &SessionsSummary{StopReasons: make(map[string]int), PaymentErrorsBy: make(map[string]int)}

	txs, err := s.repo.GetFilteredTransactions(ctx, &entity.TransactionFilter{From: &from, To: &to, SkipMeterValues: true})
	if err != nil {
		note("transactions", err)
	}
	rows := make(map[string]*SessionsRow)
	for _, tx := range txs {
		energy := sessionEnergy(tx)
		sum.Finished++
		sum.EnergyKWh += energy
		sum.BilledEUR += float64(tx.PaymentBilled) / 100
		if tx.Reason != "" {
			sum.StopReasons[tx.Reason]++
		}
		row, ok := rows[tx.ChargePointId]
		if !ok {
			row = &SessionsRow{ChargePointId: tx.ChargePointId}
			rows[tx.ChargePointId] = row
		}
		row.Sessions++
		row.EnergyKWh += energy
		if energy <= 0 {
			sum.ZeroEnergy++
			row.ZeroEnergy++
		}
		if !tx.TimeStart.IsZero() && tx.TimeStop.Sub(tx.TimeStart) < shortSessionLimit {
			sum.Short++
			row.Short++
		}
		if tx.PaymentError != "" {
			sum.PaymentErrors++
			row.PaymentErrors++
			sum.PaymentErrorsBy[templateOf(tx.PaymentError)]++
		}
		st := station(tx.ChargePointId)
		st.Sessions++
		st.EnergyKWh += energy
	}
	for _, row := range rows {
		row.EnergyKWh = round1(row.EnergyKWh)
		sum.ByChargePoint = append(sum.ByChargePoint, row)
	}
	sort.Slice(sum.ByChargePoint, func(i, j int) bool {
		return sum.ByChargePoint[i].ChargePointId < sum.ByChargePoint[j].ChargePointId
	})

	prev, err := s.repo.GetFilteredTransactions(ctx, &entity.TransactionFilter{From: &prevFrom, To: &prevTo, SkipMeterValues: true})
	if err != nil {
		note("previous transactions", err)
	}
	for _, tx := range prev {
		sum.PrevFinished++
		sum.PrevEnergyKWh += sessionEnergy(tx)
		sum.PrevBilledEUR += float64(tx.PaymentBilled) / 100
	}

	sum.EnergyKWh = round1(sum.EnergyKWh)
	sum.PrevEnergyKWh = round1(sum.PrevEnergyKWh)
	sum.BilledEUR = round2(sum.BilledEUR)
	sum.PrevBilledEUR = round2(sum.PrevBilledEUR)
	return sum
}

func sessionEnergy(tx *entity.Transaction) float64 {
	if tx.MeterStop <= tx.MeterStart {
		return 0
	}
	return float64(tx.MeterStop-tx.MeterStart) / 1000
}

func (s *Service) payments(ctx context.Context, note func(string, error)) *PaymentsState {
	ps := &PaymentsState{}
	retries, err := s.repo.GetAllPaymentRetries(ctx)
	if err != nil {
		note("payment retries", err)
		return ps
	}
	ps.RetryQueue = len(retries)
	for _, r := range retries {
		if len(ps.Retries) >= maxRetryRows {
			break
		}
		ps.Retries = append(ps.Retries, &RetryRow{
			TransactionId: r.TransactionId,
			Attempt:       r.Attempt,
			NextRetry:     r.NextRetryTime,
			LastError:     templateOf(r.LastError),
		})
	}
	return ps
}

func (s *Service) webhooks(ctx context.Context, note func(string, error)) *WebhooksState {
	ws := &WebhooksState{}
	stats, err := s.repo.GetWebhookOutboxStats(ctx)
	if err != nil {
		note("webhook stats", err)
	}
	ws.Subscribers = stats
	problems, err := s.repo.ListWebhookProblemDeliveries(ctx, 100)
	if err != nil {
		note("webhook deliveries", err)
	}
	ws.Problems = len(problems)
	for _, p := range problems {
		if len(ws.Recent) >= maxDeliveryRows {
			break
		}
		ws.Recent = append(ws.Recent, &DeliveryRow{
			Subscriber: p.Subscriber,
			Type:       p.Type,
			Status:     p.Status,
			Attempts:   p.Attempts,
			LastError:  templateOf(p.LastError),
			CreatedAt:  p.CreatedAt,
		})
	}
	return ws
}

// encodeSnapshot marshals the snapshot, cutting the longest lists until it
// fits the size limit.
func encodeSnapshot(snap *Snapshot) ([]byte, error) {
	for i := 0; ; i++ {
		data, err := json.Marshal(snap)
		if err != nil {
			return nil, err
		}
		if len(data) <= maxSnapshotBytes || i >= 8 {
			return data, nil
		}
		cut := false
		for _, d := range []*LogDigest{snap.SysLog, snap.BackendLog, snap.PaymentLog} {
			if d != nil && len(d.Events) > 10 {
				keep := len(d.Events) / 2
				d.OmittedEvents += len(d.Events) - keep
				d.Events = d.Events[:keep]
				cut = true
			}
		}
		if len(snap.ConnectorErrors) > 10 {
			snap.ConnectorErrors = snap.ConnectorErrors[:len(snap.ConnectorErrors)/2]
			cut = true
		}
		if !cut {
			return data, nil
		}
		if i == 0 {
			snap.Notes = append(snap.Notes, "lists were shortened to fit the size limit; omitted_events tells how many log shapes were left out")
		}
	}
}

func round1(v float64) float64 {
	return float64(int64(v*10+0.5)) / 10
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
