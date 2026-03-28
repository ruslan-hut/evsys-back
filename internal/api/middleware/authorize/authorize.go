package authorize

import (
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// RequirePowerUser returns middleware that rejects non-admin/operator users with 403.
func RequirePowerUser(log *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := cont.GetUser(r.Context())
			if !user.IsPowerUser() {
				log.With(
					sl.Module("middleware.authorize"),
					slog.String("user", user.Username),
					slog.String("role", user.Role),
					slog.String("path", r.URL.Path),
					slog.String("request_id", middleware.GetReqID(r.Context())),
				).Warn("access denied: not admin or operator")
				response.Forbidden(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
