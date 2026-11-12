package invoices

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/web"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
)

// Invoices serves the invoice profile of the current user.
type Invoices interface {
	GetInvoiceProfile(ctx context.Context, user *entity.User) (*entity.InvoiceProfile, error)
	SaveInvoiceProfile(ctx context.Context, user *entity.User, req *entity.InvoiceProfileRequest) (*entity.InvoiceProfile, error)
	DeleteInvoiceProfile(ctx context.Context, user *entity.User) error
}

func profileLog(logger *slog.Logger, r *http.Request, user *entity.User) *slog.Logger {
	// the profile holds a tax id and an address: only ids are logged
	return web.Log(r.Context(), logger, "handlers.invoices",
		slog.String("user", user.Username),
		sl.Secret("user_id", user.UserId),
	)
}

// GetProfile returns the profile, or an empty one with enabled=false.
func GetProfile(logger *slog.Logger, handler Invoices) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())
		log := profileLog(logger, r, user)

		profile, err := handler.GetInvoiceProfile(r.Context(), user)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to read invoice profile", err)
			return
		}
		web.OK(w, r, log, "invoice profile", profile)
	}
}

// SaveProfile creates or replaces the profile.
func SaveProfile(logger *slog.Logger, handler Invoices) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())
		log := profileLog(logger, r, user)

		var req entity.InvoiceProfileRequest
		if err := render.Bind(r, &req); err != nil {
			web.Fail(w, r, log, 400, "Invalid invoice profile", err)
			return
		}
		profile, err := handler.SaveInvoiceProfile(r.Context(), user, &req)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to save invoice profile", err)
			return
		}
		web.OK(w, r, log.With(slog.Bool("enabled", profile.Enabled)), "invoice profile saved", profile)
	}
}

// DeleteProfile removes the profile with all its data.
func DeleteProfile(logger *slog.Logger, handler Invoices) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())
		log := profileLog(logger, r, user)

		if err := handler.DeleteInvoiceProfile(r.Context(), user); err != nil {
			web.Fail(w, r, log, 0, "Failed to delete invoice profile", err)
			return
		}
		web.OK(w, r, log, "invoice profile deleted", map[string]bool{"deleted": true})
	}
}
