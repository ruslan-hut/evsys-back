package payments

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

type Payments interface {
	GetPaymentMethods(ctx context.Context, userId string) (interface{}, error)
	SavePaymentMethod(ctx context.Context, user *entity.User, pm *entity.PaymentMethod) error
	UpdatePaymentMethod(ctx context.Context, user *entity.User, pm *entity.PaymentMethod) error
	DeletePaymentMethod(ctx context.Context, user *entity.User, pm *entity.PaymentMethod) error
	SetOrder(ctx context.Context, user *entity.User, order *entity.PaymentOrder) (*entity.PaymentOrder, error)
}

func List(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.payments"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		data, err := handler.GetPaymentMethods(ctx, user.UserId)
		if err != nil {
			log.With(sl.Err(err)).Error("payment methods list")
			response.RenderErr(w, r, 204, 2001, "Failed to read payment methods", err)
			return
		}
		log.Info("payment methods list")

		render.JSON(w, r, data)
	}
}

func Save(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.payments"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		var pm entity.PaymentMethod
		if err := render.Bind(r, &pm); err != nil {
			log.With(sl.Err(err)).Error("bind")
			response.RenderErr(w, r, 400, 2001, "Failed to decode", err)
			return
		}
		log = log.With(
			slog.String("description", pm.Description),
			sl.Secret("identifier", pm.Identifier),
		)

		err := handler.SavePaymentMethod(ctx, user, &pm)
		if err != nil {
			log.With(sl.Err(err)).Error("payment method not saved")
			response.RenderErr(w, r, 204, 2001, "Failed to save payment method", err)
			return
		}
		log.Info("payment method saved")

		render.JSON(w, r, &pm)
	}
}

func Update(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.payments"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		var pm entity.PaymentMethod
		if err := render.Bind(r, &pm); err != nil {
			log.With(sl.Err(err)).Error("bind")
			response.RenderErr(w, r, 400, 2001, "Failed to decode", err)
			return
		}
		log = log.With(
			slog.String("description", pm.Description),
			sl.Secret("identifier", pm.Identifier),
		)

		err := handler.UpdatePaymentMethod(ctx, user, &pm)
		if err != nil {
			log.With(sl.Err(err)).Error("payment method not updated")
			response.RenderErr(w, r, 204, 2001, "Failed to update payment method", err)
			return
		}
		log.Info("payment method updated")

		render.JSON(w, r, &pm)
	}
}

func Delete(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.payments"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		var pm entity.PaymentMethod
		if err := render.Bind(r, &pm); err != nil {
			log.With(sl.Err(err)).Error("bind")
			response.RenderErr(w, r, 400, 2001, "Failed to decode", err)
			return
		}
		log = log.With(
			slog.String("description", pm.Description),
			sl.Secret("identifier", pm.Identifier),
		)

		err := handler.DeletePaymentMethod(ctx, user, &pm)
		if err != nil {
			log.With(sl.Err(err)).Error("payment method not deleted")
			response.RenderErr(w, r, 204, 2001, "Failed to delete payment method", err)
			return
		}
		log.Info("payment method deleted")

		render.JSON(w, r, &pm)
	}
}

func Order(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.payments"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		var order entity.PaymentOrder
		if err := render.Bind(r, &order); err != nil {
			log.With(sl.Err(err)).Error("bind")
			response.RenderErr(w, r, 400, 2001, "Failed to decode", err)
			return
		}
		log = log.With(
			slog.String("description", order.Description),
			slog.Int("transaction_id", order.TransactionId),
			sl.Secret("identifier", order.Identifier),
		)

		updated, err := handler.SetOrder(ctx, user, &order)
		if err != nil {
			log.With(sl.Err(err)).Error("order not set")
			response.RenderErr(w, r, 204, 2001, "Failed to set order", err)
			return
		}
		log.With(
			slog.Int("order", updated.Order),
		).Info("payment order set")

		render.JSON(w, r, &updated)
	}
}
