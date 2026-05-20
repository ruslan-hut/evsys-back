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

// RetryQueue is the handler dependency for the read-only payment retry queue.
type RetryQueue interface {
	GetPaymentRetryQueue(ctx context.Context, author *entity.User) ([]*entity.PaymentRetryView, error)
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
