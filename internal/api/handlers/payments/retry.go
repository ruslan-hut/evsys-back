package payments

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

// RetryQueue is the handler dependency for the payment retry queue.
type RetryQueue interface {
	GetPaymentRetryQueue(ctx context.Context, author *entity.User) ([]*entity.PaymentRetryView, error)
	ForcePaymentRetry(ctx context.Context, author *entity.User, transactionId int) error
}

// RetryQueueList serves the full payment retry queue for power users.
func RetryQueueList(logger *slog.Logger, handler RetryQueue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.payments"),
			slog.String("user", author.Username),
			sl.Secret("user_id", author.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		data, err := handler.GetPaymentRetryQueue(ctx, author)
		if err != nil {
			log.With(sl.Err(err)).Error("payment retry queue")
			response.RenderErr(w, r, 400, 2001, "Failed to read payment retry queue", err)
			return
		}
		log.Info("payment retry queue")

		render.JSON(w, r, data)
	}
}

// RetryQueueForce triggers an immediate retry of a queued payment for power users.
func RetryQueueForce(logger *slog.Logger, handler RetryQueue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.payments"),
			slog.String("user", author.Username),
			sl.Secret("user_id", author.UserId),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		idStr := chi.URLParam(r, "transactionId")
		transactionId, err := strconv.Atoi(idStr)
		if err != nil || transactionId <= 0 {
			log.With(slog.String("transaction_id", idStr)).Warn("invalid transaction id")
			response.RenderErr(w, r, 400, 2002, "Invalid transaction id", err)
			return
		}

		if err := handler.ForcePaymentRetry(ctx, author, transactionId); err != nil {
			log.With(sl.Err(err), slog.Int("transaction_id", transactionId)).Error("force payment retry")
			response.RenderErr(w, r, 400, 2003, "Failed to force payment retry", err)
			return
		}
		log.With(slog.Int("transaction_id", transactionId)).Info("force payment retry")

		render.JSON(w, r, map[string]string{"status": "ok"})
	}
}
