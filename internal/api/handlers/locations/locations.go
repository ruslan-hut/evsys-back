package locations

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

type Locations interface {
	GetLocations(ctx context.Context, accessLevel int) (interface{}, error)
	GetChargePoints(ctx context.Context, accessLevel int, search string) (interface{}, error)
	GetChargePoint(ctx context.Context, accessLevel int, id string) (interface{}, error)
	SaveChargePoint(ctx context.Context, accessLevel int, chargePoint *entity.ChargePoint) error
}

func ListLocations(logger *slog.Logger, handler Locations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.locations",
			slog.String("user", user.Username),
			slog.Int("access_level", user.AccessLevel),
		)

		data, err := handler.GetLocations(ctx, user.AccessLevel)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get locations", err)
			return
		}
		web.OK(w, r, log, "list locations", data)
	}
}

func ListChargePoints(logger *slog.Logger, handler Locations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		search := chi.URLParam(r, "search")
		log := web.Log(ctx, logger, "handlers.locations",
			slog.String("user", user.Username),
			slog.Int("access_level", user.AccessLevel),
			slog.String("search", search),
		)

		data, err := handler.GetChargePoints(ctx, user.AccessLevel, search)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get charge points", err)
			return
		}
		web.OK(w, r, log, "list charge points", data)
	}
}

func ChargePointRead(logger *slog.Logger, handler Locations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		id := chi.URLParam(r, "id")
		log := web.Log(ctx, logger, "handlers.locations",
			slog.String("user", user.Username),
			slog.Int("access_level", user.AccessLevel),
			slog.String("id", id),
		)

		data, err := handler.GetChargePoint(ctx, user.AccessLevel, id)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get charge point", err)
			return
		}
		web.OK(w, r, log, "charge point info", data)
	}
}

func ChargePointSave(logger *slog.Logger, handler Locations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		id := chi.URLParam(r, "id")
		log := web.Log(ctx, logger, "handlers.locations",
			slog.String("user", user.Username),
			slog.Int("access_level", user.AccessLevel),
			slog.String("id", id),
		)

		var chargePoint entity.ChargePoint
		if err := render.Bind(r, &chargePoint); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode charge point", err)
			return
		}

		if err := handler.SaveChargePoint(ctx, user.AccessLevel, &chargePoint); err != nil {
			web.Fail(w, r, log, 400, "Failed to save charge point", err)
			return
		}
		web.OK(w, r, log, "charge point updated", &chargePoint)
	}
}
