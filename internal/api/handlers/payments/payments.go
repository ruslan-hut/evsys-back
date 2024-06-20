package payments

import (
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"log/slog"
	"net/http"
)

type Payments interface {
	GetPaymentMethods(userId string) (interface{}, error)
	SavePaymentMethod(user *entity.User, pm *entity.PaymentMethod) error
	UpdatePaymentMethod(user *entity.User, pm *entity.PaymentMethod) error
	DeletePaymentMethod(user *entity.User, pm *entity.PaymentMethod) error
	SetOrder(user *entity.User, order *entity.PaymentOrder) (*entity.PaymentOrder, error)
}

func List(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())

		log := logger.With(
			sl.Module("handlers.payments"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		data, err := handler.GetPaymentMethods(user.UserId)
		if err != nil {
			log.Error("payment methods list", sl.Err(err))
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to read payment methods: %v", err)))
			return
		}
		log.Info("payment methods list")

		render.JSON(w, r, data)
	}
}

func Save(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())

		log := logger.With(
			sl.Module("handlers.payments"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var pm entity.PaymentMethod
		if err := render.Bind(r, &pm); err != nil {
			log.Error("bind", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode: %v", err)))
			return
		}
		log = log.With(
			slog.String("description", pm.Description),
			sl.Secret("identifier", pm.Identifier),
		)

		err := handler.SavePaymentMethod(user, &pm)
		if err != nil {
			log.Error("payment method not saved", sl.Err(err))
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to save payment method: %v", err)))
			return
		}
		log.Info("payment method saved")

		render.JSON(w, r, &pm)
	}
}

func Update(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())

		log := logger.With(
			sl.Module("handlers.payments"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var pm entity.PaymentMethod
		if err := render.Bind(r, &pm); err != nil {
			log.Error("bind", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode: %v", err)))
			return
		}
		log = log.With(
			slog.String("description", pm.Description),
			sl.Secret("identifier", pm.Identifier),
		)

		err := handler.UpdatePaymentMethod(user, &pm)
		if err != nil {
			log.Error("payment method not updated", sl.Err(err))
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to update payment method: %v", err)))
			return
		}
		log.Info("payment method updated")

		render.JSON(w, r, &pm)
	}
}

func Delete(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())

		log := logger.With(
			sl.Module("handlers.payments"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var pm entity.PaymentMethod
		if err := render.Bind(r, &pm); err != nil {
			log.Error("bind", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode: %v", err)))
			return
		}
		log = log.With(
			slog.String("description", pm.Description),
			sl.Secret("identifier", pm.Identifier),
		)

		err := handler.DeletePaymentMethod(user, &pm)
		if err != nil {
			log.Error("payment method not deleted", sl.Err(err))
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to delete payment method: %v", err)))
			return
		}
		log.Info("payment method deleted")

		render.JSON(w, r, &pm)
	}
}

func Order(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())

		log := logger.With(
			sl.Module("handlers.payments"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var order entity.PaymentOrder
		if err := render.Bind(r, &order); err != nil {
			log.Error("bind", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode: %v", err)))
			return
		}
		log = log.With(
			slog.String("description", order.Description),
			slog.Int("transaction_id", order.TransactionId),
			sl.Secret("identifier", order.Identifier),
		)

		updated, err := handler.SetOrder(user, &order)
		if err != nil {
			log.Error("order not set", sl.Err(err))
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to set order: %v", err)))
			return
		}
		log.With(
			slog.Int("order", updated.Order),
		).Info("payment order set")

		render.JSON(w, r, &updated)
	}
}
