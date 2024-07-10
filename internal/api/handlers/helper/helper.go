package helper

import (
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"log/slog"
	"net/http"
	"time"
)

type Helper interface {
	GetConfig(name string) (interface{}, error)
	GetLog(name string) (interface{}, error)
}

func Config(logger *slog.Logger, handler Helper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		name := chi.URLParam(r, "name")

		log := logger.With(
			sl.Module("handlers.helper"),
			slog.String("name", name),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		data, err := handler.GetConfig(name)
		if err != nil {
			log.Error("get config", sl.Err(err))
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get config: %v", err)))
			return
		}
		log.Info("get config success")

		render.JSON(w, r, data)
	}
}

func Log(logger *slog.Logger, handler Helper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		name := chi.URLParam(r, "name")

		log := logger.With(
			sl.Module("handlers.helper"),
			slog.String("name", name),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		data, err := handler.GetLog(name)
		if err != nil {
			log.Error("get log", sl.Err(err))
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get log: %v", err)))
			return
		}
		log.Info("get log")

		render.JSON(w, r, data)
	}
}

func OptionsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Add("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
	}
}

func Options(log *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		mod := sl.Module("middleware.options")

		fn := func(w http.ResponseWriter, r *http.Request) {
			id := middleware.GetReqID(r.Context())
			origin := r.Header.Get("Origin")
			remote := r.RemoteAddr
			// if the request is coming from a proxy, use the X-Forwarded-For header
			xRemote := r.Header.Get("X-Forwarded-For")
			if xRemote != "" {
				remote = xRemote
			}
			logger := log.With(
				mod,
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", remote),
				slog.String("request_id", id),
				slog.String("origin", origin),
			)
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			t1 := time.Now()
			defer func() {
				logger.With(
					slog.Int("status", ww.Status()),
					slog.Int("size", ww.BytesWritten()),
					slog.Float64("duration", time.Since(t1).Seconds()),
				).Info("incoming request")
			}()

			ww.Header().Add("Access-Control-Allow-Origin", origin)
			ww.Header().Add("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			ww.Header().Add("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				render.Status(r, http.StatusOK)
				return
			}

			next.ServeHTTP(ww, r)
		}

		return http.HandlerFunc(fn)
	}
}
