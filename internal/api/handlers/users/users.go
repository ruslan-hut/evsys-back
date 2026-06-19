package users

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
		ctx := r.Context()
		log := web.Log(ctx, logger, "handlers.users")

		var user entity.User
		if err := render.Bind(r, &user); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode user data", err)
			return
		}
		log = log.With(slog.String("username", user.Username))

		var data any
		var err error
		if user.Username == "" {
			data, err = handler.AuthenticateByToken(ctx, user.Password)
		} else {
			data, err = handler.AuthenticateUser(ctx, user.Username, user.Password)
		}
		if err != nil {
			web.Fail(w, r, log, 401, "Not authorized", err)
			return
		}
		web.OK(w, r, log, "user authorized", data)
	}
}

func Register(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := web.Log(ctx, logger, "handlers.users")

		var user entity.User
		if err := render.Bind(r, &user); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode user data", err)
			return
		}
		log = log.With(slog.String("username", user.Username))

		data, err := handler.AddUser(ctx, &user)
		if err != nil {
			web.Fail(w, r, log, 500, "Failed to save user", err)
			return
		}
		web.OK(w, r, log, "user registered", data)
	}
}

func Info(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		name := chi.URLParam(r, "name")
		log := web.Log(ctx, logger, "handlers.users",
			slog.String("name", name),
			slog.String("author", user.Username),
			slog.String("role", user.Role),
			slog.Int("access_level", user.AccessLevel),
		)

		data, err := handler.GetUser(ctx, user, name)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get user", err)
			return
		}
		web.OK(w, r, log, "user info", data)
	}
}

func List(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.users",
			slog.String("author", user.Username),
			slog.String("role", user.Role),
			slog.Int("access_level", user.AccessLevel),
		)

		data, err := handler.GetUsers(ctx, user)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get users", err)
			return
		}
		web.OK(w, r, log, "users list", data)
	}
}

func Create(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.users",
			slog.String("author", author.Username),
			slog.String("role", author.Role),
			slog.Int("access_level", author.AccessLevel),
		)

		var user entity.User
		if err := render.Bind(r, &user); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode user data", err)
			return
		}
		log = log.With(slog.String("new_username", user.Username))

		data, err := handler.CreateUser(ctx, author, &user)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to create user", err)
			return
		}
		web.Created(w, r, log, "user created", data)
	}
}

func Update(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		username := chi.URLParam(r, "username")
		log := web.Log(ctx, logger, "handlers.users",
			slog.String("author", author.Username),
			slog.String("target_user", username),
			slog.String("role", author.Role),
			slog.Int("access_level", author.AccessLevel),
		)

		var updates entity.UserUpdate
		if err := render.Bind(r, &updates); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode user data", err)
			return
		}

		data, err := handler.UpdateUser(ctx, author, username, &updates)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to update user", err)
			return
		}
		web.OK(w, r, log, "user updated", data)
	}
}

func Delete(logger *slog.Logger, handler Users) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		username := chi.URLParam(r, "username")
		log := web.Log(ctx, logger, "handlers.users",
			slog.String("author", author.Username),
			slog.String("target_user", username),
			slog.String("role", author.Role),
			slog.Int("access_level", author.AccessLevel),
		)

		err := handler.DeleteUser(ctx, author, username)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to delete user", err)
			return
		}
		web.OK(w, r, log, "user deleted", map[string]any{
			"success": true,
			"message": "User deleted successfully",
		})
	}
}
