package report

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/request"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"log/slog"
	"net/http"
	"time"
)

type Reports interface {
	MonthlyStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]interface{}, error)
	UsersStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]interface{}, error)
	ChargerStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]interface{}, error)
}

func MonthlyStatistics(logger *slog.Logger, handler Reports) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.report"),
			slog.String("author", user.Username),
			slog.String("role", user.Role),
			slog.Int("access_level", user.AccessLevel),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		from, err := request.GetDate(r, "from")
		if err != nil {
			log.Error("wrong parameter", sl.Err(err))
			wrongParameter(w, r, err)
			return
		}

		to, err := request.GetDate(r, "to")
		if err != nil {
			log.Error("wrong parameter", sl.Err(err))
			wrongParameter(w, r, err)
			return
		}

		group, err := request.GetString(r, "group")
		if err != nil {
			log.Error("wrong parameter", sl.Err(err))
			wrongParameter(w, r, err)
			return
		}

		log = log.With(
			slog.Time("from", from),
			slog.Time("to", to),
			slog.String("group", group),
		)

		data, err := handler.MonthlyStats(ctx, user, from, to, group)
		if err != nil {
			log.Error("get report failed", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get report data: %v", err)))
			return
		}
		log.Info("monthly report")

		render.JSON(w, r, data)
	}
}

func UsersStatistics(logger *slog.Logger, handler Reports) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.report"),
			slog.String("author", user.Username),
			slog.String("role", user.Role),
			slog.Int("access_level", user.AccessLevel),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		from, err := request.GetDate(r, "from")
		if err != nil {
			log.Error("wrong parameter", sl.Err(err))
			wrongParameter(w, r, err)
			return
		}

		to, err := request.GetDate(r, "to")
		if err != nil {
			log.Error("wrong parameter", sl.Err(err))
			wrongParameter(w, r, err)
			return
		}

		group, err := request.GetString(r, "group")
		if err != nil {
			log.Error("wrong parameter", sl.Err(err))
			wrongParameter(w, r, err)
			return
		}

		log = log.With(
			slog.Time("from", from),
			slog.Time("to", to),
			slog.String("group", group),
		)

		data, err := handler.UsersStats(ctx, user, from, to, group)
		if err != nil {
			log.Error("get report failed", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get report data: %v", err)))
			return
		}
		log.Info("users report")

		render.JSON(w, r, data)
	}
}

func ChargerStatistics(logger *slog.Logger, handler Reports) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		user := cont.GetUser(ctx)

		log := logger.With(
			sl.Module("handlers.report"),
			slog.String("author", user.Username),
			slog.String("role", user.Role),
			slog.Int("access_level", user.AccessLevel),
			slog.String("request_id", middleware.GetReqID(ctx)),
		)

		from, err := request.GetDate(r, "from")
		if err != nil {
			log.Error("wrong parameter", sl.Err(err))
			wrongParameter(w, r, err)
			return
		}

		to, err := request.GetDate(r, "to")
		if err != nil {
			log.Error("wrong parameter", sl.Err(err))
			wrongParameter(w, r, err)
			return
		}

		group, err := request.GetString(r, "group")
		if err != nil {
			log.Error("wrong parameter", sl.Err(err))
			wrongParameter(w, r, err)
			return
		}

		log = log.With(
			slog.Time("from", from),
			slog.Time("to", to),
			slog.String("group", group),
		)

		data, err := handler.ChargerStats(ctx, user, from, to, group)
		if err != nil {
			log.Error("get report failed", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to get report data: %v", err)))
			return
		}
		log.Info("charger report")

		render.JSON(w, r, data)
	}
}

func wrongParameter(w http.ResponseWriter, r *http.Request, err error) {
	render.JSON(w, r, response.Error(400, fmt.Sprintf("Invalid parameter: %v", err)))
}
