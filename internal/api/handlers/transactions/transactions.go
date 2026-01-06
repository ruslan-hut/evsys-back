package transactions

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
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

		log := logger.With(
			sl.Module("handlers.transactions"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		data, err := handler.GetActiveTransactions(ctx, user.UserId)
		if err != nil {
			log.With(sl.Err(err)).Error("active transactions")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to read transactions: %v", err)))
			return
		}
		log.Info("active transactions")

		render.JSON(w, r, data)
	}
}

func List(logger *slog.Logger, handler Transactions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		period := chi.URLParam(r, "period")

		log := logger.With(
			sl.Module("handlers.transactions"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("period", period),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		// Check for query parameters (new filtering for power users)
		filter := parseTransactionFilter(r)
		if user.IsPowerUser() && filter.HasFilters() {
			data, err := handler.GetFilteredTransactions(ctx, user, filter)
			if err != nil {
				log.With(sl.Err(err)).Error("filtered transactions list")
				render.Status(r, 204)
				render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to read transactions: %v", err)))
				return
			}
			log.With(
				slog.String("from", filter.From.String()),
				slog.String("to", filter.To.String()),
				slog.String("user", filter.Username),
				slog.String("tag", filter.IdTag),
				slog.String("charger", filter.ChargePointId),
			).Info("filtered transactions list")
			render.JSON(w, r, data)
			return
		}

		// Legacy behavior: get user's own transactions
		data, err := handler.GetTransactions(ctx, user.UserId, period)
		if err != nil {
			log.With(sl.Err(err)).Error("transactions list")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to read transactions: %v", err)))
			return
		}
		log.Info("transactions list")

		render.JSON(w, r, data)
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

	return filter
}

func Get(logger *slog.Logger, handler Transactions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		id := chi.URLParam(r, "id")

		log := logger.With(
			sl.Module("handlers.transactions"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.Int("access_level", user.AccessLevel),
			slog.String("id", id),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		transactionId, err := strconv.Atoi(id)
		if err != nil {
			log.With(sl.Err(err)).Error("transaction id")
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to parse transaction id: %v", err)))
			return
		}

		data, err := handler.GetTransaction(ctx, user.UserId, user.AccessLevel, transactionId)
		if err != nil {
			log.With(sl.Err(err)).Error("transaction info")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to read transaction info: %v", err)))
			return
		}
		log.Info("transaction info")

		render.JSON(w, r, data)
	}
}

func RecentUserChargePoints(logger *slog.Logger, handler Transactions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.transactions"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.Int("access_level", user.AccessLevel),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		data, err := handler.GetRecentChargePoints(ctx, user.UserId)
		if err != nil {
			log.With(sl.Err(err)).Error("get recent charge points")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get recent charge points: %v", err)))
			return
		}
		log.Info("list recent charge points")

		render.JSON(w, r, data)
	}
}
