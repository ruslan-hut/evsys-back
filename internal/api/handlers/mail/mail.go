package mail

import (
	"context"
	"encoding/json"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/web"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type Handler interface {
	ListMailSubscriptions(ctx context.Context, author *entity.User) ([]*entity.MailSubscription, error)
	SaveMailSubscription(ctx context.Context, author *entity.User, sub *entity.MailSubscription) (*entity.MailSubscription, error)
	DeleteMailSubscription(ctx context.Context, author *entity.User, id string) error
	SendMailSubscriptionNow(ctx context.Context, author *entity.User, id string) error
	SendTestMail(ctx context.Context, author *entity.User, to string) error
}

type testMailRequest struct {
	Email string `json:"email"`
}

func loggerWith(logger *slog.Logger, r *http.Request, author *entity.User) *slog.Logger {
	return web.Log(r.Context(), logger, "handlers.mail",
		slog.String("author", author.Username),
		slog.String("role", author.Role),
	)
}

func List(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		data, err := h.ListMailSubscriptions(ctx, author)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to list mail subscriptions", err)
			return
		}
		web.OK(w, r, log, "mail subscriptions list", data)
	}
}

func Create(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		var sub entity.MailSubscription
		if err := render.Bind(r, &sub); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode subscription", err)
			return
		}
		sub.Id = ""

		data, err := h.SaveMailSubscription(ctx, author, &sub)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to save subscription", err)
			return
		}
		web.Created(w, r, log.With(slog.String("id", data.Id)), "mail subscription created", data)
	}
}

func Update(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		id := chi.URLParam(r, "id")
		var sub entity.MailSubscription
		if err := render.Bind(r, &sub); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode subscription", err)
			return
		}
		sub.Id = id

		data, err := h.SaveMailSubscription(ctx, author, &sub)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to update subscription", err)
			return
		}
		web.OK(w, r, log.With(slog.String("id", id)), "mail subscription updated", data)
	}
}

func Delete(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		id := chi.URLParam(r, "id")
		if err := h.DeleteMailSubscription(ctx, author, id); err != nil {
			web.Fail(w, r, log, 0, "Failed to delete subscription", err)
			return
		}
		web.OK(w, r, log.With(slog.String("id", id)), "mail subscription deleted", map[string]any{"success": true})
	}
}

func Test(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		var req testMailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode request", err)
			return
		}
		if err := h.SendTestMail(ctx, author, req.Email); err != nil {
			web.Fail(w, r, log, 400, "Failed to send test mail", err)
			return
		}
		web.OK(w, r, log.With(slog.String("to", req.Email)), "test mail sent", map[string]any{"success": true})
	}
}

func SendNow(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		id := chi.URLParam(r, "id")
		if err := h.SendMailSubscriptionNow(ctx, author, id); err != nil {
			web.Fail(w, r, log, 0, "Failed to send report mail", err)
			return
		}
		web.OK(w, r, log.With(slog.String("id", id)), "mail subscription sent now", map[string]any{"success": true})
	}
}
