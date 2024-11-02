package reports

import (
	"evsys-back/internal/lib/sl"
	"log/slog"
	"time"
)

type Repository interface {
}

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

func (r *Reports) TotalsByMonth(from, to time.Time, userGroup string) (interface{}, error) {
	r.log.With(
		slog.Time("from", from),
		slog.Time("to", to),
		slog.String("userGroup", userGroup),
	).Info("totals by month")
	return nil, nil
}

func (r *Reports) TotalsByUsers(from, to time.Time, userGroup string) (interface{}, error) {
	r.log.With(
		slog.Time("from", from),
		slog.Time("to", to),
		slog.String("userGroup", userGroup),
	).Info("totals by users")
	return nil, nil
}
