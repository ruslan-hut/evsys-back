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
	GeneratedAt time.Time `json:"generated_at"`
	TimeZone    string    `json:"time_zone"`
	// Scope is present when the snapshot covers only some locations.
	Scope           *SnapshotScope   `json:"scope,omitempty"`
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

// SnapshotScope names the locations a limited snapshot covers.
type SnapshotScope struct {
	Locations    []ScopeLocation `json:"locations"`
	ChargePoints []string        `json:"charge_points"`
}

type ScopeLocation struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

// scope limits a snapshot to the charge points of some locations; the scope
// with an empty key covers the whole network.
type scope struct {
	key          string
	locationIds  []string
	names        []string
	chargePoints map[string]bool
	missing      []string
}

func (sc *scope) limited() bool { return sc.key != "" }

func (sc *scope) allows(chargePointId string) bool {
	return !sc.limited() || sc.chargePoints[chargePointId]
}

// allowsRecord admits a sys_log record of a charge point in scope, and a
// central-system record (charge point "*") that names one of them.
func (sc *scope) allowsRecord(chargePointId, text string) bool {
	if !sc.limited() {
		return true
	}
	if chargePointId != "*" && chargePointId != "" {
		return sc.chargePoints[chargePointId]
	}
	for id := range sc.chargePoints {
		if strings.Contains(text, id) {
			return true
		}
	}
	return false
}

// resolveScope finds the charge points and names of the given locations.
func (s *Service) resolveScope(ctx context.Context, locationIds []string) (*scope, error) {
	ids := entity.NormalizeLocationIds(locationIds)
	sc := &scope{key: entity.ScopeKey(ids), locationIds: ids, chargePoints: make(map[string]bool)}
	if !sc.limited() {
		return sc, nil
	}
	locations, err := s.repo.GetAllLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("read locations: %w", err)
	}
	byId := make(map[string]*entity.Location, len(locations))
	for _, l := range locations {
		byId[l.Id] = l
	}
	for _, id := range ids {
		l, ok := byId[id]
		if !ok {
			sc.missing = append(sc.missing, id)
			sc.names = append(sc.names, id)
			continue
		}
		name := l.Name
		if name == "" {
			name = l.Id
		}
		sc.names = append(sc.names, name)
		for _, cp := range l.ChargePoints {
			sc.chargePoints[cp.Id] = true
		}
	}
	return sc, nil
}

func (sc *scope) snapshotScope() *SnapshotScope {
	if !sc.limited() {
		return nil
	}
	out := &SnapshotScope{}
	for i, id := range sc.locationIds {
		out.Locations = append(out.Locations, ScopeLocation{Id: id, Name: sc.names[i]})
	}
	for id := range sc.chargePoints {
		out.ChargePoints = append(out.ChargePoints, id)
	}
	sort.Strings(out.ChargePoints)
	return out
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
	Finished      int     `json:"finished"`
	PrevFinished  int     `json:"prev_finished"`
	EnergyKWh     float64 `json:"energy_kwh"`
	PrevEnergyKWh float64 `json:"prev_energy_kwh"`
	// Money: only app sessions are paid through the system; terminal and
	// roaming sessions are settled outside it. Turnover is therefore the
	// energy delivered to clients (every session but those of internal users)
	// at the tariff price per kWh: the session's own plan, else the default.
	TariffPricePerKWhEUR    float64                  `json:"tariff_price_per_kwh_eur"`
	ClientEnergyKWh         float64                  `json:"client_energy_kwh"`
	PrevClientEnergyKWh     float64                  `json:"prev_client_energy_kwh"`
	TurnoverEstimateEUR     float64                  `json:"turnover_estimate_eur"`
	PrevTurnoverEstimateEUR float64                  `json:"prev_turnover_estimate_eur"`
	AppBilledEUR            float64                  `json:"app_billed_eur"`
	ByChannel               map[string]*ChannelUsage `json:"by_channel"`
	ZeroEnergy              int                      `json:"zero_energy"`
	Short                   int                      `json:"shorter_than_2_min"`
	PaymentErrors           int                      `json:"payment_errors"`
	StopReasons             map[string]int           `json:"stop_reasons"`
	ActiveNow               int                      `json:"active_now"`
	ByChargePoint           []*SessionsRow           `json:"by_charge_point"`
	PaymentErrorsBy         map[string]int           `json:"payment_errors_by_message,omitempty"`
}

// ChannelUsage sums the sessions of one way of paying: app, terminal (bank
// card at the charger), roaming, internal (users outside the client group),
// other (no tag information).
type ChannelUsage struct {
	Sessions            int     `json:"sessions"`
	EnergyKWh           float64 `json:"energy_kwh"`
	TurnoverEstimateEUR float64 `json:"turnover_estimate_eur"`
}

type SessionsRow struct {
	ChargePointId       string  `json:"charge_point_id"`
	Sessions            int     `json:"sessions"`
	EnergyKWh           float64 `json:"energy_kwh"`
	TurnoverEstimateEUR float64 `json:"turnover_estimate_eur"`
	ZeroEnergy          int     `json:"zero_energy"`
	Short               int     `json:"shorter_than_2_min"`
	PaymentErrors       int     `json:"payment_errors"`
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
func (s *Service) collect(ctx context.Context, from, to, prevFrom time.Time, tz string, sc *scope) *Snapshot {
	snap := &Snapshot{
		GeneratedAt:    s.now().UTC(),
		TimeZone:       tz,
		Scope:          sc.snapshotScope(),
		Period:         Period{From: from.UTC(), To: to.UTC()},
		PreviousPeriod: Period{From: prevFrom.UTC(), To: from.UTC()},
	}
	note := func(source string, err error) {
		snap.Notes = append(snap.Notes, fmt.Sprintf("%s unavailable: %v", source, err))
	}
	if sc.limited() {
		snap.Notes = append(snap.Notes, "the report covers only the locations in scope; backend and payment logs, the payment retry queue and webhooks are network-wide and left out")
		if len(sc.missing) > 0 {
			snap.Notes = append(snap.Notes, fmt.Sprintf("locations not found, so not covered: %s", strings.Join(sc.missing, ", ")))
		}
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
	if sc.limited() {
		inScope := chargePoints[:0:0]
		for _, cp := range chargePoints {
			if sc.allows(cp.Id) {
				inScope = append(inScope, cp)
			}
		}
		chargePoints = inScope
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
			if !sc.allows(r.ChargePointId) {
				continue
			}
			v := round1(r.UptimePercent)
			station(r.ChargePointId).UptimePercent = &v
		}
	}
	if rows, err := s.repo.StationUptime(ctx, prevFrom, from, ""); err != nil {
		note("previous uptime", err)
	} else {
		for _, r := range rows {
			if !sc.allows(r.ChargePointId) {
				continue
			}
			v := round1(r.UptimePercent)
			station(r.ChargePointId).PrevUptimePercent = &v
		}
	}

	if rows, err := s.repo.ErrorSummary(ctx, from, toIncl, ""); err != nil {
		note("connector errors", err)
	} else {
		rows = errorsInScope(rows, sc)
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
		for _, r := range errorsInScope(rows, sc) {
			station(r.ChargePointId).PrevConnectorErrs += r.Count
		}
	}

	sysLog, err := s.readSysLog(ctx, from, toIncl)
	full := len(sysLog) >= maxLogRecords
	sysLog = sysInScope(sysLog, sc)
	if err != nil {
		note("sys_log", err)
	} else {
		snap.SysLog = digestSysLog(sysLog, station)
		if full {
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
		for _, r := range sysInScope(prev, sc) {
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

	if sc.limited() {
		// network-wide logs, left out of a report about some locations
	} else if digest, err := s.readMessageLog(ctx, "back", from, toIncl); err != nil {
		note("back_log", err)
	} else {
		snap.BackendLog = digest
		if digest.Truncated {
			snap.Notes = append(snap.Notes, "back_log was cut at the record limit: only the newest records were read")
		}
	}
	if sc.limited() {
		// see above
	} else if digest, err := s.readMessageLog(ctx, "pay", from, toIncl); err != nil {
		note("payment_log", err)
	} else {
		snap.PaymentLog = digest
		if digest.Truncated {
			snap.Notes = append(snap.Notes, "payment_log was cut at the record limit: only the newest records were read")
		}
	}

	snap.Sessions = s.sessions(ctx, from, toIncl, prevFrom, fromIncl, sc, station, note)
	snap.Sessions.ActiveNow = activeSessions(chargePoints)
	if !sc.limited() {
		snap.Payments = s.payments(ctx, note)
		snap.Webhooks = s.webhooks(ctx, note)
	}

	// '*' marks central-system records that belong to no charge point
	delete(stations, "*")
	delete(stations, "")
	for id, st := range stations {
		if !sc.allows(id) {
			continue
		}
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

func errorsInScope(rows []*entity.ErrorSummary, sc *scope) []*entity.ErrorSummary {
	if !sc.limited() {
		return rows
	}
	var out []*entity.ErrorSummary
	for _, r := range rows {
		if sc.allows(r.ChargePointId) {
			out = append(out, r)
		}
	}
	return out
}

func sysInScope(records []entity.FeatureMessage, sc *scope) []entity.FeatureMessage {
	if !sc.limited() {
		return records
	}
	var out []entity.FeatureMessage
	for _, r := range records {
		if sc.allowsRecord(r.ChargePointId, r.Text) {
			out = append(out, r)
		}
	}
	return out
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

func (s *Service) sessions(ctx context.Context, from, to, prevFrom, prevTo time.Time, sc *scope,
	station func(string) *StationWeek, note func(string, error)) *SessionsSummary {
	sum := &SessionsSummary{
		StopReasons:     make(map[string]int),
		PaymentErrorsBy: make(map[string]int),
		ByChannel:       make(map[string]*ChannelUsage),
	}
	pricing := s.pricing(ctx, note)
	sum.TariffPricePerKWhEUR = pricing.defaultPrice

	txs, err := s.repo.GetFilteredTransactions(ctx, &entity.TransactionFilter{From: &from, To: &to, SkipMeterValues: true})
	if err != nil {
		note("transactions", err)
	}
	rows := make(map[string]*SessionsRow)
	for _, tx := range txs {
		if !sc.allows(tx.ChargePointId) {
			continue
		}
		energy := sessionEnergy(tx)
		channel, turnover := pricing.value(tx, energy)
		sum.Finished++
		sum.EnergyKWh += energy
		sum.AppBilledEUR += float64(tx.PaymentBilled) / 100
		usage, ok := sum.ByChannel[channel]
		if !ok {
			usage = &ChannelUsage{}
			sum.ByChannel[channel] = usage
		}
		usage.Sessions++
		usage.EnergyKWh += energy
		usage.TurnoverEstimateEUR += turnover
		if channel != channelInternal {
			sum.ClientEnergyKWh += energy
			sum.TurnoverEstimateEUR += turnover
		}
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
		row.TurnoverEstimateEUR += turnover
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
		row.TurnoverEstimateEUR = round2(row.TurnoverEstimateEUR)
		sum.ByChargePoint = append(sum.ByChargePoint, row)
	}
	for _, usage := range sum.ByChannel {
		usage.EnergyKWh = round1(usage.EnergyKWh)
		usage.TurnoverEstimateEUR = round2(usage.TurnoverEstimateEUR)
	}
	sort.Slice(sum.ByChargePoint, func(i, j int) bool {
		return sum.ByChargePoint[i].ChargePointId < sum.ByChargePoint[j].ChargePointId
	})

	prev, err := s.repo.GetFilteredTransactions(ctx, &entity.TransactionFilter{From: &prevFrom, To: &prevTo, SkipMeterValues: true})
	if err != nil {
		note("previous transactions", err)
	}
	for _, tx := range prev {
		if !sc.allows(tx.ChargePointId) {
			continue
		}
		energy := sessionEnergy(tx)
		channel, turnover := pricing.value(tx, energy)
		sum.PrevFinished++
		sum.PrevEnergyKWh += energy
		if channel != channelInternal {
			sum.PrevClientEnergyKWh += energy
			sum.PrevTurnoverEstimateEUR += turnover
		}
	}

	sum.EnergyKWh = round1(sum.EnergyKWh)
	sum.PrevEnergyKWh = round1(sum.PrevEnergyKWh)
	sum.ClientEnergyKWh = round1(sum.ClientEnergyKWh)
	sum.PrevClientEnergyKWh = round1(sum.PrevClientEnergyKWh)
	sum.TurnoverEstimateEUR = round2(sum.TurnoverEstimateEUR)
	sum.PrevTurnoverEstimateEUR = round2(sum.PrevTurnoverEstimateEUR)
	sum.AppBilledEUR = round2(sum.AppBilledEUR)
	return sum
}

// Ways a session is paid, as the snapshot names them.
const (
	channelApp      = "app"
	channelTerminal = "terminal"
	channelRoaming  = "roaming"
	channelInternal = "internal"
	channelOther    = "other"

	// clientGroup is the user group of paying clients ("Cliente" in the web
	// statistics); users of any other group are internal.
	clientGroup = "default"
)

// pricing values sessions at the tariff price per kWh.
type pricing struct {
	defaultPrice float64
	groups       map[string]string
}

func (s *Service) pricing(ctx context.Context, note func(string, error)) *pricing {
	p := &pricing{groups: make(map[string]string)}
	if plan, err := s.repo.GetDefaultPaymentPlan(ctx); err != nil {
		note("default tariff", err)
	} else if plan != nil {
		p.defaultPrice = float64(plan.PricePerKwh) / 100
	}
	users, err := s.repo.GetUsers(ctx)
	if err != nil {
		note("user groups", err)
	}
	for _, u := range users {
		p.groups[u.Username] = u.Group
	}
	return p
}

// value returns how a session was paid and its turnover: the energy at the
// price per kWh of the session's own plan, else of the default plan;
// nothing for internal users.
func (p *pricing) value(tx *entity.Transaction, energyKWh float64) (string, float64) {
	channel := p.channel(tx)
	if channel == channelInternal {
		return channel, 0
	}
	price := p.defaultPrice
	if tx.Plan != nil {
		price = float64(tx.Plan.PricePerKwh) / 100
	}
	return channel, energyKWh * price
}

func (p *pricing) channel(tx *entity.Transaction) string {
	username := tx.Username
	if username == "" && tx.UserTag != nil {
		username = tx.UserTag.Username
	}
	if username != "" {
		// only a group named otherwise is internal; no group is a client
		if group := p.groups[username]; group != "" && group != clientGroup {
			return channelInternal
		}
		return channelApp
	}
	if tx.UserTag != nil {
		switch tx.UserTag.Source {
		case "CC":
			return channelTerminal
		case "OCPI":
			return channelRoaming
		case "AP":
			return channelApp
		}
	}
	return channelOther
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
