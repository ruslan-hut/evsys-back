package mcpserver

import (
	"context"
	"evsys-back/entity"
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	defaultLogLimit = 100
	maxLogLimit     = 1000
)

var logNames = map[string]string{
	"sys":    "OCPP messages and connection events from charge points, written by evsys",
	"back":   "events of this backend",
	"pay":    "payment activity",
	"errors": "connector errors reported by charge points (StatusNotification with an error code)",
}

type readLogInput struct {
	Log           string `json:"log" jsonschema:"which log: sys (OCPP messages from charge points), back (backend events), pay (payment activity) or errors (connector errors)"`
	From          string `json:"from,omitempty" jsonschema:"records at or after this time"`
	To            string `json:"to,omitempty" jsonschema:"records at or before this time"`
	ChargePointId string `json:"charge_point_id,omitempty" jsonschema:"only records of this charge point (sys and errors logs)"`
	Search        string `json:"search,omitempty" jsonschema:"case-insensitive substring of the record text (on errors: info, error code or vendor error code)"`
	Category      string `json:"category,omitempty" jsonschema:"exact match on the OCPP feature for sys (e.g. StatusNotification, BootNotification, MeterValues), the category for back and pay, the error code for errors"`
	Level         string `json:"level,omitempty" jsonschema:"exact match on level for back and pay (e.g. error, warn, info). Sys records mostly carry no importance, so this filter rarely matches there"`
	Limit         int    `json:"limit,omitempty" jsonschema:"maximum number of records, newest first; default 100, at most 1000"`
}

func (t *tools) readLog(ctx context.Context, user *entity.User, in readLogInput) (any, error) {
	name := strings.ToLower(strings.TrimSpace(in.Log))
	if _, ok := logNames[name]; !ok {
		return nil, fmt.Errorf("unknown log %q: use sys, back, pay or errors", in.Log)
	}
	from, err := optionalTime(in.From, false)
	if err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	to, err := optionalTime(in.To, true)
	if err != nil {
		return nil, fmt.Errorf("to: %w", err)
	}
	n := limit(in.Limit, defaultLogLimit, maxLogLimit)
	records, err := t.core.InspectLog(ctx, user, name, &entity.LogFilter{
		From:          from,
		To:            to,
		ChargePointId: strings.TrimSpace(in.ChargePointId),
		Search:        strings.TrimSpace(in.Search),
		Category:      strings.TrimSpace(in.Category),
		Level:         strings.TrimSpace(in.Level),
		Limit:         int64(n),
	})
	if err != nil {
		return nil, err
	}
	count := 0
	if v := reflect.ValueOf(records); v.Kind() == reflect.Slice {
		count = v.Len()
	}
	result := map[string]any{
		"log":         name,
		"description": logNames[name],
		"count":       count,
		"records":     records,
	}
	if count == n {
		result["truncated"] = fmt.Sprintf("limit of %d reached: older matching records exist; narrow the period or filter further", n)
	}
	return result, nil
}

type errorSummaryInput struct {
	From          string `json:"from,omitempty" jsonschema:"start of the period; defaults to 7 days before 'to'"`
	To            string `json:"to,omitempty" jsonschema:"end of the period; defaults to now"`
	ChargePointId string `json:"charge_point_id,omitempty" jsonschema:"only errors of this charge point"`
}

func (t *tools) errorSummary(ctx context.Context, user *entity.User, in errorSummaryInput) (any, error) {
	from, to, err := period(in.From, in.To, t.now(), 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	summary, err := t.core.InspectErrorSummary(ctx, user, from, to, strings.TrimSpace(in.ChargePointId))
	if err != nil {
		return nil, err
	}
	total := 0
	for _, s := range summary {
		total += s.Count
	}
	return map[string]any{
		"from":         timestamp(from),
		"to":           timestamp(to),
		"total_errors": total,
		"groups":       summary,
		"note":         "grouped by charge point, connector, error code and vendor error code, most frequent first; read_log with log=errors lists the individual records",
	}, nil
}
