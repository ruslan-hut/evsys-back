package locations

import (
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"log/slog"
	"net/http"
)

type Locations interface {
	GetLocations(accessLevel int) (interface{}, error)
	GetChargePoints(accessLevel int, search string) (interface{}, error)
	GetChargePoint(accessLevel int, id string) (interface{}, error)
	SaveChargePoint(accessLevel int, chargePoint *entity.ChargePoint) error
}

func ListLocations(logger *slog.Logger, handler Locations) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())
		log := logger.With(
			sl.Module("handlers.locations"),
			slog.String("user", user.Username),
			slog.Int("access_level", user.AccessLevel),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		data, err := handler.GetLocations(user.AccessLevel)
		if err != nil {
			log.Error("get locations", sl.Err(err))
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
		user := cont.GetUser(r.Context())
		search := chi.URLParam(r, "search")

		log := logger.With(
			sl.Module("handlers.locations"),
			slog.String("user", user.Username),
			slog.Int("access_level", user.AccessLevel),
			slog.String("search", search),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		data, err := handler.GetChargePoints(user.AccessLevel, search)
		if err != nil {
			log.Error("get charge points", sl.Err(err))
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
		user := cont.GetUser(r.Context())
		id := chi.URLParam(r, "id")

		log := logger.With(
			sl.Module("handlers.locations"),
			slog.String("user", user.Username),
			slog.Int("access_level", user.AccessLevel),
			slog.String("id", id),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		data, err := handler.GetChargePoint(user.AccessLevel, id)
		if err != nil {
			log.Error("get charge point", sl.Err(err))
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
		user := cont.GetUser(r.Context())
		id := chi.URLParam(r, "id")

		log := logger.With(
			sl.Module("handlers.locations"),
			slog.String("user", user.Username),
			slog.Int("access_level", user.AccessLevel),
			slog.String("id", id),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var chargePoint entity.ChargePoint
		if err := render.Bind(r, &chargePoint); err != nil {
			log.Error("decode charge point", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode charge point: %v", err)))
			return
		}

		err := handler.SaveChargePoint(user.AccessLevel, &chargePoint)
		if err != nil {
			log.Error("save charge point", sl.Err(err))
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to save charge point: %v", err)))
			return
		}
		log.Info("charge point updated")

		render.JSON(w, r, &chargePoint)
	}
}
