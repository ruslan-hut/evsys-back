package payments

import (
	"context"
	"evsys-back/entity"
	"evsys-back/impl/core"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/web"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
)

type Payments interface {
	GetPaymentMethods(ctx context.Context, userId string) (any, error)
	SavePaymentMethod(ctx context.Context, user *entity.User, pm *entity.PaymentMethod) error
	UpdatePaymentMethod(ctx context.Context, user *entity.User, pm *entity.PaymentMethod) error
	DeletePaymentMethod(ctx context.Context, user *entity.User, pm *entity.PaymentMethod) error
	SetOrder(ctx context.Context, user *entity.User, order *entity.PaymentOrder) (*entity.PaymentOrder, error)
	CreateWebOrder(ctx context.Context, user *entity.User, order *entity.PaymentOrder, req *core.WebOrderRequest) (*entity.PaymentOrder, *core.EntryFormResponse, error)
}

func List(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.payments",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
		)

		data, err := handler.GetPaymentMethods(ctx, user.UserId)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to read payment methods", err)
			return
		}
		web.OK(w, r, log, "payment methods list", data)
	}
}

func Save(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.payments",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
		)

		var pm entity.PaymentMethod
		if err := render.Bind(r, &pm); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode", err)
			return
		}
		log = log.With(
			slog.String("description", pm.Description),
			sl.Secret("identifier", pm.Identifier),
		)

		err := handler.SavePaymentMethod(ctx, user, &pm)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to save payment method", err)
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
			render.JSON(w, r, map[string]any{
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
		log := web.Log(ctx, logger, "handlers.payments",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
		)

		var pm entity.PaymentMethod
		if err := render.Bind(r, &pm); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode", err)
			return
		}
		log = log.With(
			slog.String("description", pm.Description),
			sl.Secret("identifier", pm.Identifier),
		)

		err := handler.UpdatePaymentMethod(ctx, user, &pm)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to update payment method", err)
			return
		}
		web.OK(w, r, log, "payment method updated", &pm)
	}
}

func Delete(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.payments",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
		)

		var pm entity.PaymentMethod
		if err := render.Bind(r, &pm); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode", err)
			return
		}
		log = log.With(
			slog.String("description", pm.Description),
			sl.Secret("identifier", pm.Identifier),
		)

		err := handler.DeletePaymentMethod(ctx, user, &pm)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to delete payment method", err)
			return
		}
		web.OK(w, r, log, "payment method deleted", &pm)
	}
}

func Order(logger *slog.Logger, handler Payments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.payments",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
		)

		var order entity.PaymentOrder
		if err := render.Bind(r, &order); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode", err)
			return
		}
		log = log.With(
			slog.String("description", order.Description),
			slog.Int("transaction_id", order.TransactionId),
			slog.String("mode", order.Mode),
			sl.Secret("identifier", order.Identifier),
		)

		// Web / Redsys TPV Virtual hosted-form redirect flow: persist the
		// order, build and sign the Redsys entry-form parameters, and
		// return the triplet the browser must auto-POST to `form_url`.
		// The native Android flow does not send "mode" and therefore
		// stays on the legacy code path below.
		if order.Mode == "web" {
			updated, form, err := handler.CreateWebOrder(ctx, user, &order, &core.WebOrderRequest{
				UrlOk:    order.ReturnUrlOk,
				UrlKo:    order.ReturnUrlKo,
				Language: order.Language,
			})
			if err != nil {
				web.Fail(w, r, log, 400, "Failed to set order", err)
				return
			}
			web.OK(w, r, log.With(slog.Int("order", updated.Order)), "payment order set (web)", &entity.WebOrderResponse{
				PaymentOrder:       updated,
				FormUrl:            form.FormUrl,
				SignatureVersion:   form.SignatureVersion,
				MerchantParameters: form.MerchantParameters,
				Signature:          form.Signature,
			})
			return
		}

		updated, err := handler.SetOrder(ctx, user, &order)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to set order", err)
			return
		}
		web.OK(w, r, log.With(slog.Int("order", updated.Order)), "payment order set", &updated)
	}
}
