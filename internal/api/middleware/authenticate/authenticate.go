package authenticate

import (
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"evsys-back/models"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Authenticate interface {
	AuthenticateByToken(token string) (*models.User, error)
}

func New(log *slog.Logger, auth Authenticate) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		mod := sl.Module("middleware.authenticate")
		log.With(mod).Info("authenticate middleware initialized")

		fn := func(w http.ResponseWriter, r *http.Request) {
			id := middleware.GetReqID(r.Context())
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

			token := ""
			header := r.Header.Get("Authorization")
			if len(header) == 0 {
				logger = logger.With(sl.Err(fmt.Errorf("authorization header not found")))
				authFailed(ww, r, "Authorization header not found")
				return
			}
			if strings.Contains(header, "Bearer") {
				token = strings.Split(header, " ")[1]
			}
			if len(token) == 0 {
				logger = logger.With(sl.Err(fmt.Errorf("token not found")))
				authFailed(ww, r, "Token not found")
				return
			}
			logger = logger.With(sl.Secret("token", token))
			user, err := auth.AuthenticateByToken(token)
			if err != nil {
				logger = logger.With(sl.Err(err))
				authFailed(ww, r, "Unauthorized: token not found")
				return
			}
			logger = logger.With(
				slog.String("user", user.Username),
				slog.Int("access_level", user.AccessLevel),
				sl.Secret("user_id", user.UserId),
			)
			ctx := cont.PutUser(r.Context(), user)

			ww.Header().Set("X-Request-ID", id)
			next.ServeHTTP(ww, r.WithContext(ctx))
		}

		return http.HandlerFunc(fn)
	}
}

func authFailed(w http.ResponseWriter, r *http.Request, message string) {
	render.Status(r, http.StatusUnauthorized)
	render.JSON(w, r, response.Error(2001, message))
}
