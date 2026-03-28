package usertags

import (
	"context"
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



		data, err := handler.ListUserTags(ctx, author)
		if err != nil {
			log.Error("list user tags", sl.Err(err))
			response.RenderErr(w, r, 400, 2001, "Failed to list user tags", err)
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



		data, err := handler.GetUserTag(ctx, author, idTag)
		if err != nil {
			log.Error("get user tag", sl.Err(err))
			status := 400
			if err.Error() == "tag not found" {
				status = 404
			}
			response.RenderErr(w, r, status, 2001, "Failed to get user tag", err)
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



		var tag entity.UserTagCreate
		if err := render.Bind(r, &tag); err != nil {
			log.Error("decode user tag data", sl.Err(err))
			response.RenderErr(w, r, 400, 2001, "Failed to decode user tag data", err)
			return
		}
		log = log.With(slog.String("id_tag", tag.IdTag))

		data, err := handler.CreateUserTag(ctx, author, &tag)
		if err != nil {
			log.Error("create user tag", sl.Err(err))
			status := 400
			if err.Error() == "user not found" {
				status = 404
			}
			response.RenderErr(w, r, status, 2001, "Failed to create user tag", err)
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



		var updates entity.UserTagUpdate
		if err := render.Bind(r, &updates); err != nil {
			log.Error("decode user tag data", sl.Err(err))
			response.RenderErr(w, r, 400, 2001, "Failed to decode user tag data", err)
			return
		}

		data, err := handler.UpdateUserTag(ctx, author, idTag, &updates)
		if err != nil {
			log.Error("update user tag", sl.Err(err))
			status := 400
			if err.Error() == "tag not found" || err.Error() == "user not found" {
				status = 404
			}
			response.RenderErr(w, r, status, 2001, "Failed to update user tag", err)
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



		err := handler.DeleteUserTag(ctx, author, idTag)
		if err != nil {
			log.Error("delete user tag", sl.Err(err))
			status := 400
			if err.Error() == "tag not found" {
				status = 404
			}
			response.RenderErr(w, r, status, 2001, "Failed to delete user tag", err)
			return
		}
		log.Info("user tag deleted")

		render.JSON(w, r, map[string]interface{}{
			"success": true,
			"message": "Tag deleted successfully",
		})
	}
}
