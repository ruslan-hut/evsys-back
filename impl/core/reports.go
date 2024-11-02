package core

import "time"

type Reports interface {
	TotalsByMonth(from, to time.Time, userGroup string) (interface{}, error)
	TotalsByUsers(from, to time.Time, userGroup string) (interface{}, error)
}
