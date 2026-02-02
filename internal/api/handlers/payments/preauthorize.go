package payments

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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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

		log := logger.With(
			sl.Module("handlers.payments.preauthorize"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		var req entity.PreauthorizationOrderRequest
		if err := render.Bind(r, &req); err != nil {
			log.With(sl.Err(err)).Error("bind")
			render.Status(r, 400)
			render.JSON(w, r, response.Error(3001, fmt.Sprintf("Failed to decode: %v", err)))
			return
		}
		log = log.With(
			slog.Int("transaction_id", req.TransactionId),
			slog.Int("amount", req.Amount),
		)

		resp, err := handler.CreatePreauthorizationOrder(ctx, user, &req)
		if err != nil {
			log.With(sl.Err(err)).Error("create preauthorization order")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(3001, fmt.Sprintf("Failed to create order: %v", err)))
			return
		}
		log.With(slog.String("order_number", resp.OrderNumber)).Info("preauthorization order created")

		render.JSON(w, r, resp)
	}
}

// PreauthorizeSave handles POST /payment/preauthorize/save
func PreauthorizeSave(logger *slog.Logger, handler Preauthorizations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.payments.preauthorize"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		var req entity.PreauthorizationSaveRequest
		if err := render.Bind(r, &req); err != nil {
			log.With(sl.Err(err)).Error("bind")
			render.Status(r, 400)
			render.JSON(w, r, response.Error(3002, fmt.Sprintf("Failed to decode: %v", err)))
			return
		}
		log = log.With(
			slog.String("order_number", req.OrderNumber),
			slog.String("status", req.Status),
		)

		err := handler.SavePreauthorization(ctx, user, &req)
		if err != nil {
			log.With(sl.Err(err)).Error("save preauthorization")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(3002, fmt.Sprintf("Failed to save: %v", err)))
			return
		}
		log.Info("preauthorization saved")

		render.JSON(w, r, map[string]string{"status": "ok"})
	}
}

// PreauthorizeGet handles GET /payment/preauthorize/{transactionId}
func PreauthorizeGet(logger *slog.Logger, handler Preauthorizations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.payments.preauthorize"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		transactionIdStr := chi.URLParam(r, "transactionId")
		transactionId, err := strconv.Atoi(transactionIdStr)
		if err != nil {
			log.With(sl.Err(err)).Error("invalid transaction id")
			render.Status(r, 400)
			render.JSON(w, r, response.Error(3003, "Invalid transaction ID"))
			return
		}
		log = log.With(slog.Int("transaction_id", transactionId))

		preauth, err := handler.GetPreauthorization(ctx, user, transactionId)
		if err != nil {
			log.With(sl.Err(err)).Error("get preauthorization")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(3003, fmt.Sprintf("Failed to get: %v", err)))
			return
		}
		if preauth == nil {
			log.Info("preauthorization not found")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(3003, "Preauthorization not found"))
			return
		}
		log.Info("preauthorization retrieved")

		render.JSON(w, r, preauth)
	}
}

// CaptureOrder handles POST /payment/capture/order
func CaptureOrder(logger *slog.Logger, handler Preauthorizations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.payments.preauthorize"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		var req entity.CaptureOrderRequest
		if err := render.Bind(r, &req); err != nil {
			log.With(sl.Err(err)).Error("bind")
			render.Status(r, 400)
			render.JSON(w, r, response.Error(3004, fmt.Sprintf("Failed to decode: %v", err)))
			return
		}
		log = log.With(
			slog.String("order_number", req.OrderNumber),
			slog.Int("amount", req.Amount),
		)

		resp, err := handler.CapturePreauthorization(ctx, user, &req)
		if err != nil {
			log.With(sl.Err(err)).Error("capture preauthorization")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(3004, fmt.Sprintf("Failed to capture: %v", err)))
			return
		}
		log.With(
			slog.String("status", string(resp.Status)),
			slog.Int("captured_amount", resp.CapturedAmount),
		).Info("capture completed")

		render.JSON(w, r, resp)
	}
}

// PreauthorizeUpdate handles POST /payment/preauthorize/update
func PreauthorizeUpdate(logger *slog.Logger, handler Preauthorizations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.payments.preauthorize"),
			slog.String("user", user.Username),
			sl.Secret("user_id", user.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		var req entity.PreauthorizationUpdateRequest
		if err := render.Bind(r, &req); err != nil {
			log.With(sl.Err(err)).Error("bind")
			render.Status(r, 400)
			render.JSON(w, r, response.Error(3005, fmt.Sprintf("Failed to decode: %v", err)))
			return
		}
		log = log.With(
			slog.String("order_number", req.OrderNumber),
			slog.String("status", req.Status),
		)

		err := handler.UpdatePreauthorization(ctx, user, &req)
		if err != nil {
			log.With(sl.Err(err)).Error("update preauthorization")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(3005, fmt.Sprintf("Failed to update: %v", err)))
			return
		}
		log.Info("preauthorization updated")

		render.JSON(w, r, map[string]string{"status": "ok"})
	}
}
