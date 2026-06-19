package payments

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/web"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
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
		log := web.Log(ctx, logger, "handlers.payments",
			slog.String("user", author.Username),
			sl.Secret("user_id", author.UserId),
		)

		data, err := handler.GetPaymentRetryQueue(ctx, author)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to read payment retry queue", err)
			return
		}
		web.OK(w, r, log, "payment retry queue", data)
	}
}

// RetryQueueForce triggers an immediate retry of a queued payment for power users.
func RetryQueueForce(logger *slog.Logger, handler RetryQueue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.payments",
			slog.String("user", author.Username),
			sl.Secret("user_id", author.UserId),
		)

		idStr := chi.URLParam(r, "transactionId")
		transactionId, err := strconv.Atoi(idStr)
		if err != nil || transactionId <= 0 {
			web.FailCode(w, r, log, 400, 2002, "Invalid transaction id", err)
			return
		}
		log = log.With(slog.Int("transaction_id", transactionId))

		if err := handler.ForcePaymentRetry(ctx, author, transactionId); err != nil {
			web.FailCode(w, r, log, 400, 2003, "Failed to force payment retry", err)
			return
		}
		web.OK(w, r, log, "force payment retry", map[string]string{"status": "ok"})
	}
}
