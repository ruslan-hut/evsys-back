package payments

import (
	"context"
	"encoding/json"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type DirectPayments interface {
	PayTransaction(ctx context.Context, transactionId int) error
	ReturnPayment(ctx context.Context, transactionId int) error
	ReturnByOrder(ctx context.Context, orderId string, amount int) error
	Notify(ctx context.Context, data []byte) error
}

// Pay initiates a direct MIT payment for a finished transaction.
func Pay(logger *slog.Logger, handler DirectPayments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		log := logger.With(
			sl.Module("handlers.payments.pay"),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		txIdStr := chi.URLParam(r, "transactionId")
		txId, err := strconv.Atoi(txIdStr)
		if err != nil {
			log.With(slog.String("transaction_id", txIdStr)).Warn("invalid transaction id")
			response.RenderErr(w, r, 400, 3001, "Invalid transaction ID", err)
			return
		}
		log = log.With(slog.Int("transaction_id", txId))

		if err := handler.PayTransaction(ctx, txId); err != nil {
			log.With(sl.Err(err)).Error("pay transaction failed")
			response.RenderErr(w, r, 500, 3002, "Payment failed", err)
			return
		}

		log.Info("pay transaction initiated")
		response.Render(w, r, 200, 0, "ok")
	}
}

// Return initiates a full refund for a transaction.
func Return(logger *slog.Logger, handler DirectPayments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		log := logger.With(
			sl.Module("handlers.payments.return"),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		txIdStr := chi.URLParam(r, "transactionId")
		txId, err := strconv.Atoi(txIdStr)
		if err != nil {
			log.With(slog.String("transaction_id", txIdStr)).Warn("invalid transaction id")
			response.RenderErr(w, r, 400, 3001, "Invalid transaction ID", err)
			return
		}
		log = log.With(slog.Int("transaction_id", txId))

		if err := handler.ReturnPayment(ctx, txId); err != nil {
			log.With(sl.Err(err)).Error("return payment failed")
			response.RenderErr(w, r, 500, 3003, "Refund failed", err)
			return
		}

		log.Info("refund initiated")
		response.Render(w, r, 200, 0, "ok")
	}
}

// ReturnByOrder initiates a partial or full refund for a specific order.
// Expects JSON body: {"amount": 1000}
func ReturnByOrder(logger *slog.Logger, handler DirectPayments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		log := logger.With(
			sl.Module("handlers.payments.return_by_order"),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		orderId := chi.URLParam(r, "orderId")
		if orderId == "" {
			response.RenderErr(w, r, 400, 3001, "Missing order ID", nil)
			return
		}
		log = log.With(slog.String("order_id", orderId))

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.With(sl.Err(err)).Error("read body")
			response.RenderErr(w, r, 400, 3001, "Failed to read request body", err)
			return
		}

		var req struct {
			Amount int `json:"amount"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			log.With(sl.Err(err)).Error("decode body")
			response.RenderErr(w, r, 400, 3001, "Failed to decode request", err)
			return
		}

		if err := handler.ReturnByOrder(ctx, orderId, req.Amount); err != nil {
			log.With(sl.Err(err)).Error("return by order failed")
			response.RenderErr(w, r, 500, 3004, "Refund failed", err)
			return
		}

		log.With(slog.Int("amount", req.Amount)).Info("refund by order initiated")
		response.Render(w, r, 200, 0, "ok")
	}
}

// Notify handles Redsys payment webhook notifications.
// This endpoint is unauthenticated (called by Redsys).
func Notify(logger *slog.Logger, handler DirectPayments) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		log := logger.With(
			sl.Module("handlers.payments.notify"),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.With(sl.Err(err)).Error("read notification body")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := handler.Notify(ctx, body); err != nil {
			log.With(sl.Err(err)).Error("notify processing failed")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Info("payment notification processed")
		w.WriteHeader(http.StatusOK)
	}
}
