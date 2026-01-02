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
	CreateUser(ctx context.Context, author *entity.User, user *entity.User) (*entity.User, error)
	UpdateUser(ctx context.Context, author *entity.User, username string, updates *entity.UserUpdate) (*entity.User, error)
	DeleteUser(ctx context.Context, author *entity.User, username string) error
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

func Create(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.users"),
			slog.String("author", author.Username),
			slog.String("role", author.Role),
			slog.Int("access_level", author.AccessLevel),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		if !author.IsPowerUser() {
			log.Warn("access denied: not admin or operator")
			render.Status(r, 403)
			render.JSON(w, r, response.Error(2001, "Insufficient permissions"))
			return
		}

		var user entity.User
		if err := render.Bind(r, &user); err != nil {
			log.Error("decode user data", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode user data: %v", err)))
			return
		}
		log = log.With(slog.String("new_username", user.Username))

		data, err := handler.CreateUser(ctx, author, &user)
		if err != nil {
			log.Error("create user", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to create user: %v", err)))
			return
		}
		log.Info("user created")

		render.Status(r, 201)
		render.JSON(w, r, data)
	}
}

func Update(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		username := chi.URLParam(r, "username")

		log := logger.With(
			sl.Module("handlers.users"),
			slog.String("author", author.Username),
			slog.String("target_user", username),
			slog.String("role", author.Role),
			slog.Int("access_level", author.AccessLevel),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		if !author.IsPowerUser() {
			log.Warn("access denied: not admin or operator")
			render.Status(r, 403)
			render.JSON(w, r, response.Error(2001, "Insufficient permissions"))
			return
		}

		var updates entity.UserUpdate
		if err := render.Bind(r, &updates); err != nil {
			log.Error("decode user data", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode user data: %v", err)))
			return
		}

		data, err := handler.UpdateUser(ctx, author, username, &updates)
		if err != nil {
			log.Error("update user", sl.Err(err))
			if err.Error() == "user not found" {
				render.Status(r, 404)
			} else {
				render.Status(r, 400)
			}
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to update user: %v", err)))
			return
		}
		log.Info("user updated")

		render.JSON(w, r, data)
	}
}

func Delete(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		username := chi.URLParam(r, "username")

		log := logger.With(
			sl.Module("handlers.users"),
			slog.String("author", author.Username),
			slog.String("target_user", username),
			slog.String("role", author.Role),
			slog.Int("access_level", author.AccessLevel),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		if !author.IsPowerUser() {
			log.Warn("access denied: not admin or operator")
			render.Status(r, 403)
			render.JSON(w, r, response.Error(2001, "Insufficient permissions"))
			return
		}

		err := handler.DeleteUser(ctx, author, username)
		if err != nil {
			log.Error("delete user", sl.Err(err))
			if err.Error() == "user not found" {
				render.Status(r, 404)
			} else {
				render.Status(r, 400)
			}
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to delete user: %v", err)))
			return
		}
		log.Info("user deleted")

		render.JSON(w, r, map[string]interface{}{
			"success": true,
			"message": "User deleted successfully",
		})
	}
}
