package locations

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
		log := logger.With(
			sl.Module("handlers.locations"),
			slog.String("user", user.Username),
			slog.Int("access_level", user.AccessLevel),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		data, err := handler.GetLocations(ctx, user.AccessLevel)
		if err != nil {
			log.With(sl.Err(err)).Error("get locations")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get locations: %v", err)))
			return
		}
		log.Info("list locations")

		render.JSON(w, r, data)
	}
}

func ListChargePoints(logger *slog.Logger, handler Locations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		search := chi.URLParam(r, "search")

		log := logger.With(
			sl.Module("handlers.locations"),
			slog.String("user", user.Username),
			slog.Int("access_level", user.AccessLevel),
			slog.String("search", search),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		data, err := handler.GetChargePoints(ctx, user.AccessLevel, search)
		if err != nil {
			log.With(sl.Err(err)).Error("get charge points")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get charge points: %v", err)))
			return
		}
		log.Info("list charge points")

		render.JSON(w, r, data)
	}
}

func ChargePointRead(logger *slog.Logger, handler Locations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		id := chi.URLParam(r, "id")

		log := logger.With(
			sl.Module("handlers.locations"),
			slog.String("user", user.Username),
			slog.Int("access_level", user.AccessLevel),
			slog.String("id", id),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		data, err := handler.GetChargePoint(ctx, user.AccessLevel, id)
		if err != nil {
			log.With(sl.Err(err)).Error("get charge point")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get charge point: %v", err)))
			return
		}
		log.Info("charge point info")

		render.JSON(w, r, data)
	}
}

func ChargePointSave(logger *slog.Logger, handler Locations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		id := chi.URLParam(r, "id")

		log := logger.With(
			sl.Module("handlers.locations"),
			slog.String("user", user.Username),
			slog.Int("access_level", user.AccessLevel),
			slog.String("id", id),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		var chargePoint entity.ChargePoint
		if err := render.Bind(r, &chargePoint); err != nil {
			log.With(sl.Err(err)).Error("decode charge point")
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode charge point: %v", err)))
			return
		}

		err := handler.SaveChargePoint(ctx, user.AccessLevel, &chargePoint)
		if err != nil {
			log.With(sl.Err(err)).Error("save charge point")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to save charge point: %v", err)))
			return
		}
		log.Info("charge point updated")

		render.JSON(w, r, &chargePoint)
	}
}
