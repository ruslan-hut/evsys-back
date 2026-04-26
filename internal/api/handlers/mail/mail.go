package mail

import (
	"context"
	"encoding/json"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
	return logger.With(
		sl.Module("handlers.mail"),
		slog.String("author", author.Username),
		slog.String("role", author.Role),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)
}

func List(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		data, err := h.ListMailSubscriptions(ctx, author)
		if err != nil {
			log.Error("list mail subscriptions", sl.Err(err))
			response.RenderErr(w, r, 400, 2001, "Failed to list mail subscriptions", err)
			return
		}
		render.JSON(w, r, data)
	}
}

func Create(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		var sub entity.MailSubscription
		if err := render.Bind(r, &sub); err != nil {
			log.Error("decode subscription", sl.Err(err))
			response.RenderErr(w, r, 400, 2001, "Failed to decode subscription", err)
			return
		}
		sub.Id = ""

		data, err := h.SaveMailSubscription(ctx, author, &sub)
		if err != nil {
			log.Error("save subscription", sl.Err(err))
			response.RenderErr(w, r, 400, 2001, "Failed to save subscription", err)
			return
		}
		log.Info("mail subscription created", slog.String("id", data.Id))
		render.Status(r, 201)
		render.JSON(w, r, data)
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
			log.Error("decode subscription", sl.Err(err))
			response.RenderErr(w, r, 400, 2001, "Failed to decode subscription", err)
			return
		}
		sub.Id = id

		data, err := h.SaveMailSubscription(ctx, author, &sub)
		if err != nil {
			log.Error("update subscription", sl.Err(err))
			status := 400
			if err.Error() == "subscription not found" {
				status = 404
			}
			response.RenderErr(w, r, status, 2001, "Failed to update subscription", err)
			return
		}
		log.Info("mail subscription updated", slog.String("id", id))
		render.JSON(w, r, data)
	}
}

func Delete(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		id := chi.URLParam(r, "id")
		if err := h.DeleteMailSubscription(ctx, author, id); err != nil {
			log.Error("delete subscription", sl.Err(err))
			status := 400
			if err.Error() == "subscription not found" {
				status = 404
			}
			response.RenderErr(w, r, status, 2001, "Failed to delete subscription", err)
			return
		}
		log.Info("mail subscription deleted", slog.String("id", id))
		render.JSON(w, r, map[string]interface{}{"success": true})
	}
}

func Test(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		var req testMailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Error("decode test mail request", sl.Err(err))
			response.RenderErr(w, r, 400, 2001, "Failed to decode request", err)
			return
		}
		if err := h.SendTestMail(ctx, author, req.Email); err != nil {
			log.Error("send test mail", sl.Err(err))
			response.RenderErr(w, r, 400, 2001, "Failed to send test mail", err)
			return
		}
		log.Info("test mail sent", slog.String("to", req.Email))
		render.JSON(w, r, map[string]interface{}{"success": true})
	}
}

func SendNow(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		id := chi.URLParam(r, "id")
		if err := h.SendMailSubscriptionNow(ctx, author, id); err != nil {
			log.Error("send mail now", sl.Err(err))
			status := 400
			if err.Error() == "subscription not found" {
				status = 404
			}
			response.RenderErr(w, r, status, 2001, "Failed to send report mail", err)
			return
		}
		log.Info("mail subscription sent now", slog.String("id", id))
		render.JSON(w, r, map[string]interface{}{"success": true})
	}
}
