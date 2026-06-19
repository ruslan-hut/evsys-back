package report

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/request"
	"evsys-back/internal/lib/api/web"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Reports interface {
	MonthlyStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]any, error)
	UsersStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]any, error)
	ChargerStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]any, error)
	ExportStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]any, error)
	StationUptimeReport(ctx context.Context, user *entity.User, from, to time.Time, chargePointId string) ([]*entity.StationUptime, error)
	StationStatusReport(ctx context.Context, user *entity.User, chargePointId string) ([]*entity.StationStatus, error)
}

func reportLog(logger *slog.Logger, r *http.Request, user *entity.User) *slog.Logger {
	return web.Log(r.Context(), logger, "handlers.report",
		slog.String("author", user.Username),
		slog.String("role", user.Role),
		slog.Int("access_level", user.AccessLevel),
	)
}

func wrongParameter(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	web.FailCode(w, r, log, 400, 400, "Invalid parameter", err)
}

// rangeParams reads the from/to/group query parameters common to the
// statistics endpoints, returning false (after rendering an error) on failure.
func rangeParams(w http.ResponseWriter, r *http.Request, log *slog.Logger) (time.Time, time.Time, string, bool) {
	from, err := request.GetDate(r, "from")
	if err != nil {
		wrongParameter(w, r, log, err)
		return time.Time{}, time.Time{}, "", false
	}
	to, err := request.GetDate(r, "to")
	if err != nil {
		wrongParameter(w, r, log, err)
		return time.Time{}, time.Time{}, "", false
	}
	group, err := request.GetString(r, "group")
	if err != nil {
		wrongParameter(w, r, log, err)
		return time.Time{}, time.Time{}, "", false
	}
	return from, to, group, true
}

func MonthlyStatistics(logger *slog.Logger, handler Reports) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := reportLog(logger, r, user)

		from, to, group, ok := rangeParams(w, r, log)
		if !ok {
			return
		}
		log = log.With(slog.Time("from", from), slog.Time("to", to), slog.String("group", group))

		data, err := handler.MonthlyStats(ctx, user, from, to, group)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get report data", err)
			return
		}
		web.OK(w, r, log, "monthly report", data)
	}
}

func UsersStatistics(logger *slog.Logger, handler Reports) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := reportLog(logger, r, user)

		from, to, group, ok := rangeParams(w, r, log)
		if !ok {
			return
		}
		log = log.With(slog.Time("from", from), slog.Time("to", to), slog.String("group", group))

		data, err := handler.UsersStats(ctx, user, from, to, group)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get report data", err)
			return
		}
		web.OK(w, r, log, "users report", data)
	}
}

func ChargerStatistics(logger *slog.Logger, handler Reports) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := reportLog(logger, r, user)

		from, to, group, ok := rangeParams(w, r, log)
		if !ok {
			return
		}
		log = log.With(slog.Time("from", from), slog.Time("to", to), slog.String("group", group))

		data, err := handler.ChargerStats(ctx, user, from, to, group)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get report data", err)
			return
		}
		web.OK(w, r, log, "charger report", data)
	}
}

func ExportStatistics(logger *slog.Logger, handler Reports) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := reportLog(logger, r, user)

		from, to, group, ok := rangeParams(w, r, log)
		if !ok {
			return
		}
		log = log.With(slog.Time("from", from), slog.Time("to", to), slog.String("group", group))

		data, err := handler.ExportStats(ctx, user, from, to, group)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get report data", err)
			return
		}
		web.OK(w, r, log, "export report", data)
	}
}

func StationUptimeStatistics(logger *slog.Logger, handler Reports) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := reportLog(logger, r, user)

		from, err := request.GetDate(r, "from")
		if err != nil {
			wrongParameter(w, r, log, err)
			return
		}
		to, err := request.GetDate(r, "to")
		if err != nil {
			wrongParameter(w, r, log, err)
			return
		}
		if to.Before(from) {
			wrongParameter(w, r, log, fmt.Errorf("'to' must be after 'from'"))
			return
		}

		chargePointId := r.URL.Query().Get("charge_point_id")
		log = log.With(slog.Time("from", from), slog.Time("to", to), slog.String("charge_point_id", chargePointId))

		data, err := handler.StationUptimeReport(ctx, user, from, to, chargePointId)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get report data", err)
			return
		}

		// Convert to JSON-friendly format
		result := make([]entity.StationUptimeJSON, len(data))
		for i, s := range data {
			result[i] = s.ToJSON()
		}
		web.OK(w, r, log, "station uptime report", result)
	}
}

func StationStatusStatistics(logger *slog.Logger, handler Reports) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := reportLog(logger, r, user)

		chargePointId := r.URL.Query().Get("charge_point_id")
		log = log.With(slog.String("charge_point_id", chargePointId))

		data, err := handler.StationStatusReport(ctx, user, chargePointId)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to get report data", err)
			return
		}

		// Convert to JSON-friendly format
		result := make([]entity.StationStatusJSON, len(data))
		for i, s := range data {
			result[i] = s.ToJSON()
		}
		web.OK(w, r, log, "station status report", result)
	}
}
