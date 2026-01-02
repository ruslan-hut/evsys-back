package usertags

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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

		log := logger.With(
			sl.Module("handlers.usertags"),
			slog.String("author", author.Username),
			slog.String("role", author.Role),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		if !author.IsPowerUser() {
			log.Warn("access denied: not admin or operator")
			render.Status(r, 403)
			render.JSON(w, r, response.Error(2001, "Insufficient permissions"))
			return
		}

		data, err := handler.ListUserTags(ctx, author)
		if err != nil {
			log.Error("list user tags", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to list user tags: %v", err)))
			return
		}
		log.Info("user tags list")

		render.JSON(w, r, data)
	}
}

func Info(logger *slog.Logger, handler UserTags) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		idTag := chi.URLParam(r, "idTag")

		log := logger.With(
			sl.Module("handlers.usertags"),
			slog.String("author", author.Username),
			slog.String("id_tag", idTag),
			slog.String("role", author.Role),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		if !author.IsPowerUser() {
			log.Warn("access denied: not admin or operator")
			render.Status(r, 403)
			render.JSON(w, r, response.Error(2001, "Insufficient permissions"))
			return
		}

		data, err := handler.GetUserTag(ctx, author, idTag)
		if err != nil {
			log.Error("get user tag", sl.Err(err))
			if err.Error() == "tag not found" {
				render.Status(r, 404)
			} else {
				render.Status(r, 400)
			}
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get user tag: %v", err)))
			return
		}
		log.Info("user tag info")

		render.JSON(w, r, data)
	}
}

func Create(logger *slog.Logger, handler UserTags) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.usertags"),
			slog.String("author", author.Username),
			slog.String("role", author.Role),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		if !author.IsPowerUser() {
			log.Warn("access denied: not admin or operator")
			render.Status(r, 403)
			render.JSON(w, r, response.Error(2001, "Insufficient permissions"))
			return
		}

		var tag entity.UserTagCreate
		if err := render.Bind(r, &tag); err != nil {
			log.Error("decode user tag data", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode user tag data: %v", err)))
			return
		}
		log = log.With(slog.String("id_tag", tag.IdTag))

		data, err := handler.CreateUserTag(ctx, author, &tag)
		if err != nil {
			log.Error("create user tag", sl.Err(err))
			if err.Error() == "user not found" {
				render.Status(r, 404)
			} else {
				render.Status(r, 400)
			}
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to create user tag: %v", err)))
			return
		}
		log.Info("user tag created")

		render.Status(r, 201)
		render.JSON(w, r, data)
	}
}

func Update(logger *slog.Logger, handler UserTags) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		idTag := chi.URLParam(r, "idTag")

		log := logger.With(
			sl.Module("handlers.usertags"),
			slog.String("author", author.Username),
			slog.String("id_tag", idTag),
			slog.String("role", author.Role),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		if !author.IsPowerUser() {
			log.Warn("access denied: not admin or operator")
			render.Status(r, 403)
			render.JSON(w, r, response.Error(2001, "Insufficient permissions"))
			return
		}

		var updates entity.UserTagUpdate
		if err := render.Bind(r, &updates); err != nil {
			log.Error("decode user tag data", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode user tag data: %v", err)))
			return
		}

		data, err := handler.UpdateUserTag(ctx, author, idTag, &updates)
		if err != nil {
			log.Error("update user tag", sl.Err(err))
			if err.Error() == "tag not found" || err.Error() == "user not found" {
				render.Status(r, 404)
			} else {
				render.Status(r, 400)
			}
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to update user tag: %v", err)))
			return
		}
		log.Info("user tag updated")

		render.JSON(w, r, data)
	}
}

func Delete(logger *slog.Logger, handler UserTags) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		idTag := chi.URLParam(r, "idTag")

		log := logger.With(
			sl.Module("handlers.usertags"),
			slog.String("author", author.Username),
			slog.String("id_tag", idTag),
			slog.String("role", author.Role),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		if !author.IsPowerUser() {
			log.Warn("access denied: not admin or operator")
			render.Status(r, 403)
			render.JSON(w, r, response.Error(2001, "Insufficient permissions"))
			return
		}

		err := handler.DeleteUserTag(ctx, author, idTag)
		if err != nil {
			log.Error("delete user tag", sl.Err(err))
			if err.Error() == "tag not found" {
				render.Status(r, 404)
			} else {
				render.Status(r, 400)
			}
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to delete user tag: %v", err)))
			return
		}
		log.Info("user tag deleted")

		render.JSON(w, r, map[string]interface{}{
			"success": true,
			"message": "Tag deleted successfully",
		})
	}
}
