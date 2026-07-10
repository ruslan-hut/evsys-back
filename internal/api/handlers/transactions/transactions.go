package transactions

import (
	"context"
	"encoding/json"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/web"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type Transactions interface {
	GetActiveTransactions(ctx context.Context, userId string) (any, error)
	GetTransactions(ctx context.Context, userId, period string) (any, error)
	GetFilteredTransactions(ctx context.Context, user *entity.User, filter *entity.TransactionFilter) (any, error)
	GetTransaction(ctx context.Context, userId string, accessLevel, id int) (any, error)
	GetRecentChargePoints(ctx context.Context, userId string) (any, error)
	SendTransactionMail(ctx context.Context, author *entity.User, transactionId int, to string) error
	GetTransactionReceipt(ctx context.Context, author *entity.User, transactionId int) (string, error)
}

type sendMailRequest struct {
	Email string `json:"email"`
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

// SendMail emails the full transaction data to the address in the request body.
func SendMail(logger *slog.Logger, handler Transactions) http.HandlerFunc {
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

		var req sendMailRequest
		if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode request", err)
			return
		}

		if err = handler.SendTransactionMail(ctx, user, transactionId, req.Email); err != nil {
			web.Fail(w, r, log, 0, "Failed to send transaction mail", err)
			return
		}
		web.OK(w, r, log.With(sl.Secret("to", req.Email)), "transaction mail sent",
			map[string]any{"success": true})
	}
}

// Receipt returns the printable HTML document for a transaction. The browser
// loads it into a frame and prints it, so the body is HTML rather than JSON.
func Receipt(logger *slog.Logger, handler Transactions) http.HandlerFunc {
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

		document, err := handler.GetTransactionReceipt(ctx, user, transactionId)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to render transaction receipt", err)
			return
		}

		log.Info("transaction receipt")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// The document embeds another user's data only for power users, and is
		// never a page the browser should treat as a top-level navigation target.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition",
			fmt.Sprintf("inline; filename=\"transaction-%d.html\"", transactionId))
		_, _ = w.Write([]byte(document))
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
