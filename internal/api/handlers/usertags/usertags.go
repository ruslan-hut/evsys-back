package usertags

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

type UserTags interface {
	ListUserTags(ctx context.Context, author *entity.User) ([]*entity.UserTag, error)
	GetUserTag(ctx context.Context, author *entity.User, idTag string) (*entity.UserTag, error)
	CreateUserTag(ctx context.Context, author *entity.User, tag *entity.UserTagCreate) (*entity.UserTag, error)
	UpdateUserTag(ctx context.Context, author *entity.User, idTag string, updates *entity.UserTagUpdate) (*entity.UserTag, error)
	DeleteUserTag(ctx context.Context, author *entity.User, idTag string) error
}

func List(logger *slog.Logger, handler UserTags) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.usertags",
			slog.String("author", author.Username),
			slog.String("role", author.Role),
		)

		data, err := handler.ListUserTags(ctx, author)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to list user tags", err)
			return
		}
		web.OK(w, r, log, "user tags list", data)
	}
}

func Info(logger *slog.Logger, handler UserTags) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		idTag := chi.URLParam(r, "idTag")
		log := web.Log(ctx, logger, "handlers.usertags",
			slog.String("author", author.Username),
			slog.String("id_tag", idTag),
			slog.String("role", author.Role),
		)

		data, err := handler.GetUserTag(ctx, author, idTag)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to get user tag", err)
			return
		}
		web.OK(w, r, log, "user tag info", data)
	}
}

func Create(logger *slog.Logger, handler UserTags) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.usertags",
			slog.String("author", author.Username),
			slog.String("role", author.Role),
		)

		var tag entity.UserTagCreate
		if err := render.Bind(r, &tag); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode user tag data", err)
			return
		}
		log = log.With(slog.String("id_tag", tag.IdTag))

		data, err := handler.CreateUserTag(ctx, author, &tag)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to create user tag", err)
			return
		}
		web.Created(w, r, log, "user tag created", data)
	}
}

func Update(logger *slog.Logger, handler UserTags) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		idTag := chi.URLParam(r, "idTag")
		log := web.Log(ctx, logger, "handlers.usertags",
			slog.String("author", author.Username),
			slog.String("id_tag", idTag),
			slog.String("role", author.Role),
		)

		var updates entity.UserTagUpdate
		if err := render.Bind(r, &updates); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode user tag data", err)
			return
		}

		data, err := handler.UpdateUserTag(ctx, author, idTag, &updates)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to update user tag", err)
			return
		}
		web.OK(w, r, log, "user tag updated", data)
	}
}

func Delete(logger *slog.Logger, handler UserTags) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		idTag := chi.URLParam(r, "idTag")
		log := web.Log(ctx, logger, "handlers.usertags",
			slog.String("author", author.Username),
			slog.String("id_tag", idTag),
			slog.String("role", author.Role),
		)

		err := handler.DeleteUserTag(ctx, author, idTag)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to delete user tag", err)
			return
		}
		web.OK(w, r, log, "user tag deleted", map[string]interface{}{
			"success": true,
			"message": "Tag deleted successfully",
		})
	}
}
