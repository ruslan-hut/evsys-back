package users

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

type Users interface {
	AuthenticateByToken(ctx context.Context, token string) (*entity.User, error)
	AuthenticateUser(ctx context.Context, username, password string) (*entity.User, error)
	AddUser(ctx context.Context, user *entity.User) (*entity.User, error)
	GetUser(ctx context.Context, author *entity.User, username string) (*entity.UserInfo, error)
	GetUsers(ctx context.Context, user *entity.User) ([]*entity.User, error)
}

func Authenticate(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.With(
			sl.Module("handlers.users"),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var user entity.User
		if err := render.Bind(r, &user); err != nil {
			log.Error("decode user data", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode user data: %v", err)))
			return
		}
		log = log.With(slog.String("username", user.Username))

		ctx := r.Context()
		var data interface{}
		var err error

		if user.Username == "" {
			data, err = handler.AuthenticateByToken(ctx, user.Password)
		} else {
			data, err = handler.AuthenticateUser(ctx, user.Username, user.Password)
		}

		if err != nil {
			log.Error("not authorized", sl.Err(err))
			render.Status(r, 401)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Not authorized: %v", err)))
			return
		}
		log.Info("user authorized")

		render.JSON(w, r, data)
	}
}

func Register(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.With(
			sl.Module("handlers.users"),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var user entity.User
		if err := render.Bind(r, &user); err != nil {
			log.Error("decode user data", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode user data: %v", err)))
			return
		}
		log = log.With(slog.String("username", user.Username))

		ctx := r.Context()
		data, err := handler.AddUser(ctx, &user)
		if err != nil {
			log.Error("save user", sl.Err(err))
			render.Status(r, 500)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to save user: %v", err)))
			return
		}
		log.Info("user registered")

		render.JSON(w, r, data)
	}
}

func Info(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		user := cont.GetUser(ctx)
		name := chi.URLParam(r, "name")

		log := logger.With(
			sl.Module("handlers.users"),
			slog.String("name", name),
			slog.String("author", user.Username),
			slog.String("role", user.Role),
			slog.Int("access_level", user.AccessLevel),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		data, err := handler.GetUser(ctx, user, name)
		if err != nil {
			log.Error("get user", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get user: %v", err)))
			return
		}
		log.Info("user info")

		render.JSON(w, r, data)
	}
}

func List(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.users"),
			slog.String("author", user.Username),
			slog.String("role", user.Role),
			slog.Int("access_level", user.AccessLevel),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		data, err := handler.GetUsers(ctx, user)
		if err != nil {
			log.Error("get users", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get users: %v", err)))
			return
		}
		log.Info("users list")

		render.JSON(w, r, data)
	}
}
