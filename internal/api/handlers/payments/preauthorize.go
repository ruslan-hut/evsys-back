package payments

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/api/web"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// Preauthorizations interface for preauthorization operations
type Preauthorizations interface {
	CreatePreauthorizationOrder(ctx context.Context, user *entity.User, req *entity.PreauthorizationOrderRequest) (*entity.PreauthorizationOrderResponse, error)
	SavePreauthorization(ctx context.Context, user *entity.User, req *entity.PreauthorizationSaveRequest) error
	GetPreauthorization(ctx context.Context, user *entity.User, transactionId int) (*entity.Preauthorization, error)
	CapturePreauthorization(ctx context.Context, user *entity.User, req *entity.CaptureOrderRequest) (*entity.CaptureOrderResponse, error)
	UpdatePreauthorization(ctx context.Context, user *entity.User, req *entity.PreauthorizationUpdateRequest) error
}

// PreauthorizeOrder handles POST /payment/preauthorize/order
func PreauthorizeOrder(logger *slog.Logger, handler Preauthorizations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.payments.preauthorize",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
		)

		var req entity.PreauthorizationOrderRequest
		if err := render.Bind(r, &req); err != nil {
			web.FailCode(w, r, log, 400, 3001, "Failed to decode", err)
			return
		}
		log = log.With(
			slog.Int("transaction_id", req.TransactionId),
			slog.Int("amount", req.Amount),
		)

		resp, err := handler.CreatePreauthorizationOrder(ctx, user, &req)
		if err != nil {
			web.FailCode(w, r, log, 400, 3001, "Failed to create order", err)
			return
		}
		web.OK(w, r, log.With(slog.Int("order", resp.Order)), "preauthorization order created", resp)
	}
}

// PreauthorizeSave handles POST /payment/preauthorize/save
func PreauthorizeSave(logger *slog.Logger, handler Preauthorizations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.payments.preauthorize",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
		)

		var req entity.PreauthorizationSaveRequest
		if err := render.Bind(r, &req); err != nil {
			web.FailCode(w, r, log, 400, 3002, "Failed to decode", err)
			return
		}
		log = log.With(
			slog.String("order_number", req.OrderNumber),
			slog.String("status", req.Status),
		)

		err := handler.SavePreauthorization(ctx, user, &req)
		if err != nil {
			web.FailCode(w, r, log, 400, 3002, "Failed to save", err)
			return
		}
		web.OK(w, r, log, "preauthorization saved", map[string]string{"status": "ok"})
	}
}

// PreauthorizeGet handles GET /payment/preauthorize/{transactionId}
func PreauthorizeGet(logger *slog.Logger, handler Preauthorizations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.payments.preauthorize",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
		)

		transactionIdStr := chi.URLParam(r, "transactionId")
		transactionId, err := strconv.Atoi(transactionIdStr)
		if err != nil {
			log.With(sl.Err(err)).Error("invalid transaction id")
			response.Render(w, r, 400, 3003, "Invalid transaction ID")
			return
		}
		log = log.With(slog.Int("transaction_id", transactionId))

		preauth, err := handler.GetPreauthorization(ctx, user, transactionId)
		if err != nil {
			web.FailCode(w, r, log, 400, 3003, "Failed to get", err)
			return
		}
		if preauth == nil {
			log.Info("preauthorization not found")
			response.Render(w, r, 404, 3003, "Preauthorization not found")
			return
		}
		web.OK(w, r, log, "preauthorization retrieved", preauth)
	}
}

// CaptureOrder handles POST /payment/capture/order
func CaptureOrder(logger *slog.Logger, handler Preauthorizations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.payments.preauthorize",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
		)

		var req entity.CaptureOrderRequest
		if err := render.Bind(r, &req); err != nil {
			web.FailCode(w, r, log, 400, 3004, "Failed to decode", err)
			return
		}
		log = log.With(
			slog.String("original_order", req.OriginalOrder),
			slog.Int("amount", req.Amount),
		)

		resp, err := handler.CapturePreauthorization(ctx, user, &req)
		if err != nil {
			web.FailCode(w, r, log, 400, 3004, "Failed to capture", err)
			return
		}
		web.OK(w, r, log.With(
			slog.String("status", string(resp.Status)),
			slog.Int("amount", resp.Amount),
		), "capture completed", resp)
	}
}

// PreauthorizeUpdate handles POST /payment/preauthorize/update
func PreauthorizeUpdate(logger *slog.Logger, handler Preauthorizations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.payments.preauthorize",
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
		)

		var req entity.PreauthorizationUpdateRequest
		if err := render.Bind(r, &req); err != nil {
			web.FailCode(w, r, log, 400, 3005, "Failed to decode", err)
			return
		}
		log = log.With(
			slog.String("order_number", req.OrderNumber),
			slog.String("status", req.Status),
		)

		err := handler.UpdatePreauthorization(ctx, user, &req)
		if err != nil {
			web.FailCode(w, r, log, 400, 3005, "Failed to update", err)
			return
		}
		web.OK(w, r, log, "preauthorization updated", map[string]string{"status": "ok"})
	}
}
