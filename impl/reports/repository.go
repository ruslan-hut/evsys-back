package reports

import "time"

type Repository interface {
	TotalsByMonth(from, to time.Time, userGroup string) ([]interface{}, error)
}
