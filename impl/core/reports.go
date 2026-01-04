package core

import (
	"context"
	"evsys-back/entity"
	"time"
)

type Reports interface {
	TotalsByMonth(ctx context.Context, from, to time.Time, userGroup string) ([]interface{}, error)
	TotalsByUsers(ctx context.Context, from, to time.Time, userGroup string) ([]interface{}, error)
	TotalsByCharger(ctx context.Context, from, to time.Time, userGroup string) ([]interface{}, error)

	// Station uptime reports
	StationUptime(ctx context.Context, from, to time.Time, chargePointId string) ([]*entity.StationUptime, error)
	StationStatus(ctx context.Context, chargePointId string) ([]*entity.StationStatus, error)
}
