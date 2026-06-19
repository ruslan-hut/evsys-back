package helper

import (
	"context"
	"evsys-back/internal/lib/api/web"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type Helper interface {
	GetConfig(ctx context.Context, name string) (any, error)
	GetLog(ctx context.Context, name string) (any, error)
}

func Config(logger *slog.Logger, handler Helper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		name := chi.URLParam(r, "name")
		log := web.Log(ctx, logger, "handlers.helper", slog.String("name", name))

		data, err := handler.GetConfig(ctx, name)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get config", err)
			return
		}
		web.OK(w, r, log, "get config success", data)
	}
}

func Log(logger *slog.Logger, handler Helper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		name := chi.URLParam(r, "name")
		log := web.Log(ctx, logger, "handlers.helper", slog.String("name", name))

		data, err := handler.GetLog(ctx, name)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get log", err)
			return
		}
		web.OK(w, r, log, "get log", data)
	}
}

func Options() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		fn := func(w http.ResponseWriter, r *http.Request) {

			w.Header().Add("Access-Control-Allow-Origin", r.Header.Get("Origin"))
			w.Header().Add("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				render.Status(r, http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	}
}
