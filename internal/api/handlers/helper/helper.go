package helper

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/web"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type Helper interface {
	GetConfig(ctx context.Context, name string) (any, error)
	GetLog(ctx context.Context, name string, filter *entity.LogFilter) (any, error)
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
		filter := parseLogFilter(r)
		log := web.Log(ctx, logger, "handlers.helper",
			slog.String("name", name),
			slog.String("charger", filter.ChargePointId),
		)

		data, err := handler.GetLog(ctx, name, filter)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get log", err)
			return
		}
		web.OK(w, r, log, "get log", data)
	}
}

// parseLogFilter extracts optional log filter parameters from the query string.
// Absent or unparsable values leave the corresponding criteria unset, so a
// request without parameters keeps the legacy behaviour.
func parseLogFilter(r *http.Request) *entity.LogFilter {
	filter := &entity.LogFilter{}

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, ok := parseLogTime(fromStr); ok {
			filter.From = &t
		}
	}

	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, ok := parseLogTime(toStr); ok {
			// a bare date means the whole day
			if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && len(toStr) == len("2006-01-02") {
				t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			}
			filter.To = &t
		}
	}

	filter.ChargePointId = r.URL.Query().Get("charge_point_id")

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.ParseInt(limitStr, 10, 64); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}

	return filter
}

func parseLogTime(value string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
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
