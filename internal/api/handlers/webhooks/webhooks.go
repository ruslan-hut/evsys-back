package webhooks

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/web"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type Handler interface {
	ListWebhookSubscribers(ctx context.Context, author *entity.User) ([]*entity.WebhookSubscriber, error)
	SaveWebhookSubscriber(ctx context.Context, author *entity.User, sub *entity.WebhookSubscriber) (*entity.WebhookSubscriber, error)
	DeleteWebhookSubscriber(ctx context.Context, author *entity.User, id string) error
	GetWebhookHealth(ctx context.Context, author *entity.User) ([]*entity.WebhookHealthView, error)
	ListWebhookFailures(ctx context.Context, author *entity.User) ([]*entity.WebhookDeliveryView, error)
}

func loggerWith(logger *slog.Logger, r *http.Request, author *entity.User) *slog.Logger {
	return web.Log(r.Context(), logger, "handlers.webhooks",
		slog.String("author", author.Username),
		slog.String("role", author.Role),
	)
}

func List(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		data, err := h.ListWebhookSubscribers(ctx, author)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to list webhook subscribers", err)
			return
		}
		web.OK(w, r, log, "webhook subscribers list", data)
	}
}

func Create(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		var sub entity.WebhookSubscriber
		if err := render.Bind(r, &sub); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode subscriber", err)
			return
		}
		sub.Id = ""

		data, err := h.SaveWebhookSubscriber(ctx, author, &sub)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to save subscriber", err)
			return
		}
		web.Created(w, r, log.With(slog.String("id", data.Id)), "webhook subscriber created", data)
	}
}

func Update(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		id := chi.URLParam(r, "id")
		var sub entity.WebhookSubscriber
		if err := render.Bind(r, &sub); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode subscriber", err)
			return
		}
		sub.Id = id

		data, err := h.SaveWebhookSubscriber(ctx, author, &sub)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to update subscriber", err)
			return
		}
		web.OK(w, r, log.With(slog.String("id", id)), "webhook subscriber updated", data)
	}
}

func Delete(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		id := chi.URLParam(r, "id")
		if err := h.DeleteWebhookSubscriber(ctx, author, id); err != nil {
			web.Fail(w, r, log, 0, "Failed to delete subscriber", err)
			return
		}
		web.OK(w, r, log.With(slog.String("id", id)), "webhook subscriber deleted", map[string]any{"success": true})
	}
}

func Health(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		data, err := h.GetWebhookHealth(ctx, author)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to read webhook health", err)
			return
		}
		web.OK(w, r, log, "webhook health", data)
	}
}

func Failures(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		data, err := h.ListWebhookFailures(ctx, author)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to list webhook failures", err)
			return
		}
		web.OK(w, r, log, "webhook failures", data)
	}
}
