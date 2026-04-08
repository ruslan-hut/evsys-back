package payments

import (
	"context"
	"evsys-back/entity"
	"evsys-back/impl/core"
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
	CreateInSiteOrder(ctx context.Context, user *entity.User, order *entity.PaymentOrder) (*entity.PaymentOrder, *core.InSiteTokenizationParams, error)
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
			response.RenderErr(w, r, 400, 2001, "Failed to read payment methods", err)
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
			response.RenderErr(w, r, 400, 2001, "Failed to save payment method", err)
			return
		}
		log.Info("payment method saved")

		// The web inSite flow asks for the updated list of cards so it can
		// refresh its UI in a single round-trip. Android does not set this
		// flag and keeps receiving the single PaymentMethod object.
		if r.URL.Query().Get("include_list") == "1" {
			methods, listErr := handler.GetPaymentMethods(ctx, user.UserId)
			if listErr != nil {
				log.With(sl.Err(listErr)).Warn("saved but failed to load updated list")
				render.JSON(w, r, &pm)
				return
			}
			render.JSON(w, r, map[string]interface{}{
				"saved":   &pm,
				"methods": methods,
			})
			return
		}

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
			response.RenderErr(w, r, 400, 2001, "Failed to update payment method", err)
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
			response.RenderErr(w, r, 400, 2001, "Failed to delete payment method", err)
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
			slog.String("mode", order.Mode),
			sl.Secret("identifier", order.Identifier),
		)

		// Web / Redsys inSite flow: sign the merchant parameters server-side
		// and return them together with the stored order so the browser can
		// drive the inSite JS SDK. The native Android flow does not send
		// "mode" and therefore stays on the legacy code path below.
		if order.Mode == "insite" {
			updated, params, err := handler.CreateInSiteOrder(ctx, user, &order)
			if err != nil {
				log.With(sl.Err(err)).Error("inSite order not set")
				response.RenderErr(w, r, 400, 2001, "Failed to set order", err)
				return
			}
			log.With(
				slog.Int("order", updated.Order),
			).Info("payment order set (inSite)")
			render.JSON(w, r, &entity.InSiteOrderResponse{
				PaymentOrder:       updated,
				SignatureVersion:   params.SignatureVersion,
				MerchantParameters: params.MerchantParameters,
				Signature:          params.Signature,
				MerchantCode:       params.MerchantCode,
				Terminal:           params.Terminal,
				OrderNumber:        params.OrderNumber,
			})
			return
		}

		updated, err := handler.SetOrder(ctx, user, &order)
		if err != nil {
			log.With(sl.Err(err)).Error("order not set")
			response.RenderErr(w, r, 400, 2001, "Failed to set order", err)
			return
		}
		log.With(
			slog.Int("order", updated.Order),
		).Info("payment order set")

		render.JSON(w, r, &updated)
	}
}
