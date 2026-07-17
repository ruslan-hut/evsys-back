package core

import (
	"context"
	"evsys-back/entity"
	"time"
)

type Reports interface {
	TotalsByMonth(ctx context.Context, from, to time.Time, userGroup string) ([]any, error)
	TotalsByUsers(ctx context.Context, from, to time.Time, userGroup string) ([]any, error)
	TotalsByCharger(ctx context.Context, from, to time.Time, userGroup string) ([]any, error)

	TotalsByHour(ctx context.Context, from, to time.Time, userGroup string) ([]any, error)

	// Power reports
	PowerStats(ctx context.Context, from, to time.Time, chargePointId, userGroup, groupBy string) ([]*entity.PowerStats, error)

	// Station uptime reports
	StationUptime(ctx context.Context, from, to time.Time, chargePointId string) ([]*entity.StationUptime, error)
	StationStatus(ctx context.Context, chargePointId string) ([]*entity.StationStatus, error)
}
