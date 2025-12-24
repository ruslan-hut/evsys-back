package core

import (
	"context"
	"time"
)

type Reports interface {
	TotalsByMonth(ctx context.Context, from, to time.Time, userGroup string) ([]interface{}, error)
	TotalsByUsers(ctx context.Context, from, to time.Time, userGroup string) ([]interface{}, error)
	TotalsByCharger(ctx context.Context, from, to time.Time, userGroup string) ([]interface{}, error)
}
