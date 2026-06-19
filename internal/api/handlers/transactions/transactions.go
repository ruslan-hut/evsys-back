package transactions

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/web"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type Transactions interface {
	GetActiveTransactions(ctx context.Context, userId string) (interface{}, error)
	GetTransactions(ctx context.Context, userId, period string) (interface{}, error)
	GetFilteredTransactions(ctx context.Context, user *entity.User, filter *entity.TransactionFilter) (interface{}, error)
	GetTransaction(ctx context.Context, userId string, accessLevel, id int) (interface{}, error)
	GetRecentChargePoints(ctx context.Context, userId string) (interface{}, error)
}

func ListActive(logger *slog.Logger, handler Transactions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.transactions",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
		)

		data, err := handler.GetActiveTransactions(ctx, user.UserId)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to read transactions", err)
			return
		}
		web.OK(w, r, log, "active transactions", data)
	}
}

func List(logger *slog.Logger, handler Transactions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		period := chi.URLParam(r, "period")
		log := web.Log(ctx, logger, "handlers.transactions",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("period", period),
		)

		// Check for query parameters (new filtering for power users)
		filter := parseTransactionFilter(r)
		if user.IsPowerUser() && filter.HasFilters() {
			data, err := handler.GetFilteredTransactions(ctx, user, filter)
			if err != nil {
				web.Fail(w, r, log, 400, "Failed to read transactions", err)
				return
			}
			web.OK(w, r, log.With(
				slog.String("from", filter.From.String()),
				slog.String("to", filter.To.String()),
				slog.String("user", filter.Username),
				slog.String("tag", filter.IdTag),
				slog.String("charger", filter.ChargePointId),
			), "filtered transactions list", data)
			return
		}

		// Legacy behavior: get user's own transactions
		data, err := handler.GetTransactions(ctx, user.UserId, period)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to read transactions", err)
			return
		}
		web.OK(w, r, log, "transactions list", data)
	}
}

// parseTransactionFilter extracts filter parameters from query string
func parseTransactionFilter(r *http.Request) *entity.TransactionFilter {
	filter := &entity.TransactionFilter{}

	// Parse 'from' date (format: YYYY-MM-DD)
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			filter.From = &t
		}
	}

	// Parse 'to' date (format: YYYY-MM-DDTHH:mm:ss or YYYY-MM-DD)
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		// Try full datetime format first
		if t, err := time.Parse("2006-01-02T15:04:05", toStr); err == nil {
			filter.To = &t
		} else if t, err := time.Parse("2006-01-02", toStr); err == nil {
			// If just date, set to end of day
			endOfDay := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filter.To = &endOfDay
		}
	}

	filter.Username = r.URL.Query().Get("username")
	filter.IdTag = r.URL.Query().Get("id_tag")
	filter.ChargePointId = r.URL.Query().Get("charge_point_id")
	filter.WithError = r.URL.Query().Get("with_error") == "true"

	return filter
}

func Get(logger *slog.Logger, handler Transactions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		id := chi.URLParam(r, "id")
		log := web.Log(ctx, logger, "handlers.transactions",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.Int("access_level", user.AccessLevel),
			slog.String("id", id),
		)

		transactionId, err := strconv.Atoi(id)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to parse transaction id", err)
			return
		}

		data, err := handler.GetTransaction(ctx, user.UserId, user.AccessLevel, transactionId)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to read transaction info", err)
			return
		}
		web.OK(w, r, log, "transaction info", data)
	}
}

func RecentUserChargePoints(logger *slog.Logger, handler Transactions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.transactions",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.Int("access_level", user.AccessLevel),
		)

		data, err := handler.GetRecentChargePoints(ctx, user.UserId)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get recent charge points", err)
			return
		}
		web.OK(w, r, log, "list recent charge points", data)
	}
}
