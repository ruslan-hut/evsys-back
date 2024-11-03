package reports

import (
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

func (r *Reports) TotalsByMonth(from, to time.Time, userGroup string) ([]interface{}, error) {
	log := r.log.With(
		slog.Time("from", from),
		slog.Time("to", to),
		slog.String("userGroup", userGroup),
	)
	data, err := r.repo.TotalsByMonth(from, to, userGroup)
	if err != nil {
		log.Error("totals by month failed", sl.Err(err))
		return []interface{}{}, nil
	}
	if data == nil {
		log.Debug("totals by month: no data")
		return []interface{}{}, nil
	}
	log.With(
		slog.Int("count", len(data)),
	).Debug("totals by month")
	return data, nil
}

func (r *Reports) TotalsByUsers(from, to time.Time, userGroup string) ([]interface{}, error) {
	log := r.log.With(
		slog.Time("from", from),
		slog.Time("to", to),
		slog.String("userGroup", userGroup),
	)
	data, err := r.repo.TotalsByUsers(from, to, userGroup)
	if err != nil {
		log.Error("totals by users failed", sl.Err(err))
		return []interface{}{}, nil
	}
	if data == nil {
		log.Debug("totals by users: no data")
		return []interface{}{}, nil
	}
	log.With(
		slog.Int("count", len(data)),
	).Debug("totals by users")
	return nil, nil
}
