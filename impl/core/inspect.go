package core

import (
	"context"
	"evsys-back/entity"
	"fmt"
	"time"
)

// The Inspect methods serve the MCP server: read-only access for admins and
// operators to the data the web UI shows them. Unlike the REST read paths they
// return stored values as they are - connectors of an offline charge point keep
// their last reported status, meter values are not resampled unless asked -
// because the caller is analysing the system rather than rendering it.

// InspectLocations returns all locations. As in GetLocations, only users with
// the maximum access level see them.
func (c *Core) InspectLocations(ctx context.Context, author *entity.User) ([]*entity.Location, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	if author.AccessLevel < MaxAccessLevel {
		return nil, fmt.Errorf("access denied: locations need access level %d", MaxAccessLevel)
	}
	return c.repo.GetLocations(ctx)
}

// InspectChargePoints returns charge points matching the search term, within
// the author's access level, with their connectors.
func (c *Core) InspectChargePoints(ctx context.Context, author *entity.User, search string) ([]*entity.ChargePoint, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	return c.repo.GetChargePoints(ctx, author.AccessLevel, search)
}

// InspectChargePoint returns one charge point with its connectors.
func (c *Core) InspectChargePoint(ctx context.Context, author *entity.User, id string) (*entity.ChargePoint, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	cp, err := c.repo.GetChargePoint(ctx, author.AccessLevel, id)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, fmt.Errorf("charge point %s %w", id, entity.ErrNotFound)
	}
	return cp, nil
}

// InspectActiveTransactions returns every running transaction, of all users.
func (c *Core) InspectActiveTransactions(ctx context.Context, author *entity.User) ([]*entity.ChargeState, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	states, err := c.repo.GetAllActiveTransactions(ctx, author.AccessLevel)
	if err != nil {
		return nil, err
	}
	for _, state := range states {
		state.CheckState()
	}
	return states, nil
}

// InspectTransactions returns finished transactions matching the filter,
// newest first.
func (c *Core) InspectTransactions(ctx context.Context, author *entity.User, filter *entity.TransactionFilter) ([]*entity.Transaction, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	if filter == nil {
		filter = &entity.TransactionFilter{}
	}
	transactions, err := c.repo.GetFilteredTransactions(ctx, filter)
	if err != nil {
		return nil, err
	}
	if transactions == nil {
		transactions = make([]*entity.Transaction, 0)
	}
	return transactions, nil
}

// InspectTransaction returns the full state of one transaction, running or
// finished, with every meter value recorded for it.
func (c *Core) InspectTransaction(ctx context.Context, author *entity.User, id int) (*entity.ChargeState, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	state, err := c.repo.GetTransactionState(ctx, "", author.AccessLevel, id)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, fmt.Errorf("transaction %d %w", id, entity.ErrNotFound)
	}
	state.CheckState()
	return state, nil
}

// InspectLog reads one of the logs: sys (OCPP events from evsys), back (this
// service), pay (payment activity) or errors (connector errors from evsys).
func (c *Core) InspectLog(ctx context.Context, author *entity.User, name string, filter *entity.LogFilter) (any, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	return c.repo.ReadLog(ctx, name, filter)
}

// InspectErrorSummary counts connector errors per charge point, connector and
// error code over a period.
func (c *Core) InspectErrorSummary(ctx context.Context, author *entity.User, from, to time.Time, chargePointId string) ([]*entity.ErrorSummary, error) {
	if err := c.requirePowerUser(author); err != nil {
		return nil, err
	}
	summary, err := c.repo.ErrorSummary(ctx, from, to, chargePointId)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		summary = make([]*entity.ErrorSummary, 0)
	}
	return summary, nil
}
