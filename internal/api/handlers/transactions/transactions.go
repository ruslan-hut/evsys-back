package transactions

import (
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

type Transactions interface {
	GetActiveTransactions(userId string) (interface{}, error)
	GetTransactions(userId, period string) (interface{}, error)
	GetTransaction(userId string, accessLevel, id int) (interface{}, error)
	GetRecentChargePoints(userId string) (interface{}, error)
}

func ListActive(logger *slog.Logger, handler Transactions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())

		log := logger.With(
			sl.Module("handlers.transactions"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		data, err := handler.GetActiveTransactions(user.UserId)
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
		user := cont.GetUser(r.Context())
		period := chi.URLParam(r, "period")

		log := logger.With(
			sl.Module("handlers.transactions"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("period", period),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		data, err := handler.GetTransactions(user.UserId, period)
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

func Get(logger *slog.Logger, handler Transactions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())
		id := chi.URLParam(r, "id")

		log := logger.With(
			sl.Module("handlers.transactions"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.Int("access_level", user.AccessLevel),
			slog.String("id", id),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		transactionId, err := strconv.Atoi(id)
		if err != nil {
			log.With(sl.Err(err)).Error("transaction id")
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to parse transaction id: %v", err)))
			return
		}

		data, err := handler.GetTransaction(user.UserId, user.AccessLevel, transactionId)
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
		user := cont.GetUser(r.Context())

		log := logger.With(
			sl.Module("handlers.transactions"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.Int("access_level", user.AccessLevel),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		data, err := handler.GetRecentChargePoints(user.UserId)
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
