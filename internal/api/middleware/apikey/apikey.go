package apikey

import (
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

// New returns middleware that validates a Bearer API key from the Authorization header.
// Used to authenticate service-to-service calls (e.g. central system -> payment endpoints).
func New(log *slog.Logger, key string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := log.With(
				sl.Module("middleware.apikey"),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("request_id", middleware.GetReqID(r.Context())),
			)

			header := r.Header.Get("Authorization")
			if header == "" {
				logger.Warn("missing authorization header")
				response.Render(w, r, http.StatusUnauthorized, 2001, "Authorization header not found")
				return
			}

			var token string
			if strings.HasPrefix(header, "Bearer ") {
				token = header[7:]
			}
			if token == "" {
				logger.Warn("missing bearer token")
				response.Render(w, r, http.StatusUnauthorized, 2001, "Bearer token not found")
				return
			}

			if token != key {
				logger.Warn("invalid api key")
				response.Render(w, r, http.StatusUnauthorized, 2001, "Invalid API key")
				return
			}

			logger.Debug("api key authenticated")
			next.ServeHTTP(w, r)
		})
	}
}
