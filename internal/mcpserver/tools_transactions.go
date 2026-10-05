package mcpserver

import (
	"context"
	"evsys-back/entity"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	defaultTransactionLimit = 100
	maxTransactionLimit     = 1000
	defaultMeterPoints      = 120
	maxMeterPoints          = 2000
)

func (t *tools) listActiveTransactions(ctx context.Context, user *entity.User, _ noInput) (any, error) {
	sessions, err := t.core.InspectActiveTransactions(ctx, user)
	if err != nil {
		return nil, err
	}
	now := t.now()
	totalPower := 0
	views := make([]activeSessionView, 0, len(sessions))
	for _, s := range sessions {
		view := activeSessionFrom(s, now)
		totalPower += view.PowerW
		views = append(views, view)
	}
	return map[string]any{
		"count":         len(views),
		"total_power_w": totalPower,
		"sessions":      views,
	}, nil
}

type searchTransactionsInput struct {
	From             string `json:"from,omitempty" jsonschema:"sessions that ended at or after this time; defaults to 30 days before 'to'"`
	To               string `json:"to,omitempty" jsonschema:"sessions that ended at or before this time; defaults to now"`
	ChargePointId    string `json:"charge_point_id,omitempty" jsonschema:"only sessions on this charge point"`
	Username         string `json:"username,omitempty" jsonschema:"only sessions of this user (all of their RFID tags)"`
	IdTag            string `json:"id_tag,omitempty" jsonschema:"only sessions started with this RFID tag"`
	WithPaymentError bool   `json:"with_payment_error,omitempty" jsonschema:"only sessions whose payment failed"`
	Limit            int    `json:"limit,omitempty" jsonschema:"maximum number of sessions, newest first; default 100, at most 1000. Totals cover the returned sessions only"`
}

func (t *tools) searchTransactions(ctx context.Context, user *entity.User, in searchTransactionsInput) (any, error) {
	from, to, err := period(in.From, in.To, t.now(), 30*24*time.Hour)
	if err != nil {
		return nil, err
	}
	n := limit(in.Limit, defaultTransactionLimit, maxTransactionLimit)
	transactions, err := t.core.InspectTransactions(ctx, user, &entity.TransactionFilter{
		From:            &from,
		To:              &to,
		Username:        strings.TrimSpace(in.Username),
		IdTag:           strings.TrimSpace(in.IdTag),
		ChargePointId:   strings.TrimSpace(in.ChargePointId),
		WithError:       in.WithPaymentError,
		Limit:           int64(n),
		SkipMeterValues: true,
	})
	if err != nil {
		return nil, err
	}

	type totals struct {
		Sessions      int     `json:"sessions"`
		EnergyWh      int     `json:"energy_wh"`
		DurationHours float64 `json:"duration_h"`
		PaymentAmount int     `json:"payment_amount"`
		PaymentBilled int     `json:"payment_billed"`
	}
	var sum totals
	rows := make([]transactionRow, 0, len(transactions))
	for _, tr := range transactions {
		row := transactionRowFrom(tr)
		sum.Sessions++
		sum.EnergyWh += row.EnergyWh
		sum.DurationHours += row.DurationMinutes / 60
		sum.PaymentAmount += row.PaymentAmount
		sum.PaymentBilled += row.PaymentBilled
		rows = append(rows, row)
	}
	sum.DurationHours = math.Round(sum.DurationHours*10) / 10

	result := map[string]any{
		"from":         timestamp(from),
		"to":           timestamp(to),
		"count":        len(rows),
		"totals":       sum,
		"transactions": rows,
	}
	if len(rows) == n {
		result["truncated"] = fmt.Sprintf("limit of %d reached: older sessions in the period are not included; narrow the period or use energy_report for totals", n)
	}
	return result, nil
}

type getTransactionInput struct {
	TransactionId  int `json:"transaction_id" jsonschema:"the transaction id"`
	MaxMeterPoints int `json:"max_meter_points,omitempty" jsonschema:"return at most this many meter values, evenly spaced over the session with the first and last always included; default 120, at most 2000. meter_value_count tells how many were recorded"`
}

func (t *tools) getTransaction(ctx context.Context, user *entity.User, in getTransactionInput) (any, error) {
	if in.TransactionId <= 0 {
		return nil, fmt.Errorf("transaction_id is required")
	}
	points := limit(in.MaxMeterPoints, defaultMeterPoints, maxMeterPoints)
	state, err := t.core.InspectTransaction(ctx, user, in.TransactionId)
	if err != nil {
		return nil, err
	}
	end := state.TimeStop
	if !state.IsFinished {
		end = t.now()
	}
	return transactionDetail{
		ChargeState:     state,
		TimeStart:       timestamp(state.TimeStarted),
		TimeStop:        timestamp(state.TimeStop),
		DurationMinutes: minutes(end.Sub(state.TimeStarted)),
		PaymentMethod:   paymentMethodFrom(state.PaymentMethod),
		PaymentOrders:   paymentOrdersFrom(state.PaymentOrders),
		MeterValueCount: len(state.MeterValues),
		MeterValues:     metersFrom(sample(state.MeterValues, points)),
	}, nil
}

// sample picks at most n evenly spaced readings, always keeping the first and
// the last. Readings are picked, not averaged, so every returned point is one
// the charger actually reported.
func sample(values []entity.TransactionMeter, n int) []entity.TransactionMeter {
	if n <= 0 || len(values) <= n {
		return values
	}
	if n == 1 {
		return values[len(values)-1:]
	}
	picked := make([]entity.TransactionMeter, 0, n)
	step := float64(len(values)-1) / float64(n-1)
	for i := 0; i < n; i++ {
		picked = append(picked, values[int(math.Round(float64(i)*step))])
	}
	return picked
}
