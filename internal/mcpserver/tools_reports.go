package mcpserver

import (
	"context"
	"evsys-back/entity"
	"fmt"
	"strings"
	"time"
)

const (
	defaultUserGroup       = "default"
	defaultMinSessions     = 2
	defaultMaxSegments     = 300
	maxConcurrencySegments = 2000
)

type energyReportInput struct {
	GroupBy   string `json:"group_by" jsonschema:"month, user, charger or hour"`
	From      string `json:"from,omitempty" jsonschema:"sessions that ended at or after this time; defaults to 30 days before 'to'"`
	To        string `json:"to,omitempty" jsonschema:"sessions that ended at or before this time; defaults to now"`
	UserGroup string `json:"user_group,omitempty" jsonschema:"user group whose sessions are counted, as in the web statistics page; default 'default'"`
}

var energyNotes = map[string]string{
	"month":   "one row per month: total = energy in Wh, count = sessions, average = mean energy per session in Wh",
	"user":    "one row per user: total = energy in Wh, count = sessions, average = mean energy per session in Wh",
	"charger": "one row per charge point (named in the 'user' field): total = energy in Wh, count = sessions, average = mean energy per session in Wh",
	"hour":    "energy in Wh of the sessions that ended in each hour (UTC), by date and hour",
}

func (t *tools) energyReport(ctx context.Context, user *entity.User, in energyReportInput) (any, error) {
	from, to, err := period(in.From, in.To, t.now(), 30*24*time.Hour)
	if err != nil {
		return nil, err
	}
	group := strings.TrimSpace(in.UserGroup)
	if group == "" {
		group = defaultUserGroup
	}
	groupBy := strings.ToLower(strings.TrimSpace(in.GroupBy))
	var rows []any
	switch groupBy {
	case "month":
		rows, err = t.core.MonthlyStats(ctx, user, from, to, group)
	case "user":
		rows, err = t.core.UsersStats(ctx, user, from, to, group)
	case "charger":
		rows, err = t.core.ChargerStats(ctx, user, from, to, group)
	case "hour":
		rows, err = t.core.ExportStats(ctx, user, from, to, group)
	default:
		return nil, fmt.Errorf("group_by=%q: expected month, user, charger or hour", in.GroupBy)
	}
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = make([]any, 0)
	}
	return map[string]any{
		"from":       timestamp(from),
		"to":         timestamp(to),
		"user_group": group,
		"group_by":   groupBy,
		"rows":       rows,
		"note":       energyNotes[groupBy] + ". Only sessions of users in the group are counted.",
	}, nil
}

type powerReportInput struct {
	GroupBy       string `json:"group_by,omitempty" jsonschema:"charger (default), session, hour or day"`
	From          string `json:"from,omitempty" jsonschema:"sessions that ended at or after this time; defaults to 7 days before 'to'"`
	To            string `json:"to,omitempty" jsonschema:"sessions that ended at or before this time; defaults to now"`
	ChargePointId string `json:"charge_point_id,omitempty" jsonschema:"only this charge point"`
	UserGroup     string `json:"user_group,omitempty" jsonschema:"only sessions of users in this group; empty counts every session"`
}

func (t *tools) powerReport(ctx context.Context, user *entity.User, in powerReportInput) (any, error) {
	from, to, err := period(in.From, in.To, t.now(), 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	grouping, ok := entity.PowerGroupingFromString(strings.ToLower(strings.TrimSpace(in.GroupBy)))
	if !ok {
		return nil, fmt.Errorf("group_by=%q: expected charger, session, hour or day", in.GroupBy)
	}
	rows, err := t.core.PowerStatsReport(ctx, user, from, to, strings.TrimSpace(in.ChargePointId), strings.TrimSpace(in.UserGroup), string(grouping))
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = make([]*entity.PowerStats, 0)
	}
	return map[string]any{
		"from":     timestamp(from),
		"to":       timestamp(to),
		"group_by": grouping,
		"rows":     rows,
		"note":     "Energy in Wh, power in W. Charger and session rows: total_consumed = meter_stop - meter_start, avg_charging_power = mean of the samples drawing power, avg_session_power = energy over the whole session duration (lowered by idle time), max_power = highest single sample. Hour and day rows: total_consumed is integrated from per-minute fleet power inside the bucket, avg_charging_power = mean concurrent fleet power while charging, max_power = peak concurrent fleet draw. Power figures are meaningless where samples is 0",
	}, nil
}

type stationUptimeInput struct {
	From          string `json:"from,omitempty" jsonschema:"start of the period; defaults to 7 days before 'to'"`
	To            string `json:"to,omitempty" jsonschema:"end of the period; defaults to now"`
	ChargePointId string `json:"charge_point_id,omitempty" jsonschema:"only this charge point"`
}

func (t *tools) stationUptime(ctx context.Context, user *entity.User, in stationUptimeInput) (any, error) {
	from, to, err := period(in.From, in.To, t.now(), 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	stats, err := t.core.StationUptimeReport(ctx, user, from, to, strings.TrimSpace(in.ChargePointId))
	if err != nil {
		return nil, err
	}
	rows := make([]entity.StationUptimeJSON, 0, len(stats))
	for _, s := range stats {
		rows = append(rows, s.ToJSON())
	}
	return map[string]any{
		"from":     timestamp(from),
		"to":       timestamp(to),
		"stations": rows,
		"note":     "online time is derived from the connect/disconnect events of enabled charge points in the sys log; a period starting before the first recorded event is shortened to it",
	}, nil
}

type stationStatusInput struct {
	ChargePointId string `json:"charge_point_id,omitempty" jsonschema:"only this charge point"`
}

func (t *tools) stationStatus(ctx context.Context, user *entity.User, in stationStatusInput) (any, error) {
	stats, err := t.core.StationStatusReport(ctx, user, strings.TrimSpace(in.ChargePointId))
	if err != nil {
		return nil, err
	}
	rows := make([]entity.StationStatusJSON, 0, len(stats))
	for _, s := range stats {
		rows = append(rows, s.ToJSON())
	}
	return map[string]any{
		"stations": rows,
		"note":     "state and since come from the last connect/disconnect event of each enabled charge point",
	}, nil
}

type siteConcurrencyInput struct {
	From        string `json:"from,omitempty" jsonschema:"start of the window; defaults to 7 days before 'to'"`
	To          string `json:"to,omitempty" jsonschema:"end of the window; defaults to now"`
	LocationId  string `json:"location_id,omitempty" jsonschema:"only this location"`
	MinSessions int    `json:"min_sessions,omitempty" jsonschema:"list only the segments with at least this many sessions charging at once; default 2 (overlaps), 1 lists the whole timeline"`
	MaxSegments int    `json:"max_segments,omitempty" jsonschema:"maximum number of segments listed per location; default 300, at most 2000. Summary figures are exact regardless"`
}

func (t *tools) siteConcurrency(ctx context.Context, user *entity.User, in siteConcurrencyInput) (any, error) {
	from, to, err := period(in.From, in.To, t.now(), 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	minSessions := in.MinSessions
	if minSessions <= 0 {
		minSessions = defaultMinSessions
	}
	maxSegments := limit(in.MaxSegments, defaultMaxSegments, maxConcurrencySegments)
	sites, err := t.core.SiteConcurrencyReport(ctx, user, from, to, strings.TrimSpace(in.LocationId), minSessions, maxSegments)
	if err != nil {
		return nil, err
	}
	if sites == nil {
		sites = make([]*entity.SiteConcurrency, 0)
	}
	return map[string]any{
		"from":         timestamp(from),
		"to":           timestamp(to),
		"min_sessions": minSessions,
		"locations":    sites,
		"note":         "peak_assigned_amps is the sum of limits the load balancer permitted (a plan); peak_power_watts is what the chargers measured (actual draw). Locations with neither a session nor measured power in the window are omitted",
	}, nil
}
