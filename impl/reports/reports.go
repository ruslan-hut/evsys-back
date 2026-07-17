package reports

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"time"
)

type Reports struct {
	repo Repository
	log  *slog.Logger
}

func New(repo Repository, log *slog.Logger) *Reports {
	if repo == nil {
		log.Error("reports init failed: nil repository")
		return nil
	}
	return &Reports{
		repo: repo,
		log:  log.With(sl.Module("impl.reports")),
	}
}

func (r *Reports) TotalsByMonth(ctx context.Context, from, to time.Time, userGroup string) ([]any, error) {
	log := r.log.With(
		slog.Time("from", from),
		slog.Time("to", to),
		slog.String("userGroup", userGroup),
	)
	data, err := r.repo.TotalsByMonth(ctx, from, to, userGroup)
	if err != nil {
		log.Error("totals by month failed", sl.Err(err))
		return []any{}, nil
	}
	if data == nil {
		log.Debug("totals by month: no data")
		return []any{}, nil
	}
	log.With(
		slog.Int("count", len(data)),
	).Debug("totals by month")
	return data, nil
}

func (r *Reports) TotalsByUsers(ctx context.Context, from, to time.Time, userGroup string) ([]any, error) {
	log := r.log.With(
		slog.Time("from", from),
		slog.Time("to", to),
		slog.String("userGroup", userGroup),
	)
	data, err := r.repo.TotalsByUsers(ctx, from, to, userGroup)
	if err != nil {
		log.Error("totals by users failed", sl.Err(err))
		return []any{}, nil
	}
	if data == nil {
		log.Debug("totals by users: no data")
		return []any{}, nil
	}
	log.With(
		slog.Int("count", len(data)),
	).Debug("totals by users")
	return data, nil
}

func (r *Reports) TotalsByCharger(ctx context.Context, from, to time.Time, userGroup string) ([]any, error) {
	log := r.log.With(
		slog.Time("from", from),
		slog.Time("to", to),
		slog.String("userGroup", userGroup),
	)
	data, err := r.repo.TotalsByCharger(ctx, from, to, userGroup)
	if err != nil {
		log.Error("totals by charger failed", sl.Err(err))
		return []any{}, nil
	}
	if data == nil {
		log.Debug("totals by charger: no data")
		return []any{}, nil
	}
	log.With(
		slog.Int("count", len(data)),
	).Debug("totals by charger")
	return data, nil
}

func (r *Reports) TotalsByHour(ctx context.Context, from, to time.Time, userGroup string) ([]any, error) {
	log := r.log.With(
		slog.Time("from", from),
		slog.Time("to", to),
		slog.String("userGroup", userGroup),
	)
	data, err := r.repo.TotalsByHour(ctx, from, to, userGroup)
	if err != nil {
		log.Error("totals by hour failed", sl.Err(err))
		return []any{}, nil
	}
	if data == nil {
		log.Debug("totals by hour: no data")
		return []any{}, nil
	}
	log.With(
		slog.Int("count", len(data)),
	).Debug("totals by hour")
	return data, nil
}

func (r *Reports) PowerStats(ctx context.Context, from, to time.Time, chargePointId, userGroup, groupBy string) ([]*entity.PowerStats, error) {
	log := r.log.With(
		slog.Time("from", from),
		slog.Time("to", to),
		slog.String("chargePointId", chargePointId),
		slog.String("userGroup", userGroup),
		slog.String("groupBy", groupBy),
	)
	data, err := r.repo.PowerStats(ctx, from, to, chargePointId, userGroup, groupBy)
	if err != nil {
		log.Error("power stats failed", sl.Err(err))
		return nil, err
	}
	if data == nil {
		log.Debug("power stats: no data")
		return []*entity.PowerStats{}, nil
	}
	log.With(
		slog.Int("count", len(data)),
	).Debug("power stats")
	return data, nil
}

func (r *Reports) StationUptime(ctx context.Context, from, to time.Time, chargePointId string) ([]*entity.StationUptime, error) {
	log := r.log.With(
		slog.Time("from", from),
		slog.Time("to", to),
		slog.String("chargePointId", chargePointId),
	)
	data, err := r.repo.StationUptime(ctx, from, to, chargePointId)
	if err != nil {
		log.Error("station uptime failed", sl.Err(err))
		return nil, err
	}
	if data == nil {
		log.Debug("station uptime: no data")
		return []*entity.StationUptime{}, nil
	}
	log.With(
		slog.Int("count", len(data)),
	).Debug("station uptime")
	return data, nil
}

func (r *Reports) StationStatus(ctx context.Context, chargePointId string) ([]*entity.StationStatus, error) {
	log := r.log.With(
		slog.String("chargePointId", chargePointId),
	)
	data, err := r.repo.StationStatus(ctx, chargePointId)
	if err != nil {
		log.Error("station status failed", sl.Err(err))
		return nil, err
	}
	if data == nil {
		log.Debug("station status: no data")
		return []*entity.StationStatus{}, nil
	}
	log.With(
		slog.Int("count", len(data)),
	).Debug("station status")
	return data, nil
}
