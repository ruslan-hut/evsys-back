package aireport

import (
	"evsys-back/entity"
	"fmt"
	"time"
	_ "time/tzdata" // the schedule must not depend on the host's zoneinfo
)

// catchUpWindow is how late a scheduled run may still start, after a restart
// or an outage that covered its slot.
const catchUpWindow = 6 * time.Hour

func location(s *entity.AIReportSettings) (*time.Location, error) {
	loc, err := time.LoadLocation(s.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("time zone %q: %w", s.TimeZone, err)
	}
	return loc, nil
}

// slotOn returns the scheduled moment on the given calendar day in loc.
func slotOn(year int, month time.Month, day, hour int, loc *time.Location) time.Time {
	return time.Date(year, month, day, hour, 0, 0, 0, loc)
}

// lastSlot returns the latest scheduled moment at or before now.
func lastSlot(now time.Time, weekday time.Weekday, hour int, loc *time.Location) time.Time {
	t := now.In(loc)
	for i := 0; i <= 7; i++ {
		c := slotOn(t.Year(), t.Month(), t.Day()-i, hour, loc)
		if c.Weekday() == weekday && !c.After(t) {
			return c
		}
	}
	// unreachable: one of eight consecutive days is the weekday at or before now
	return slotOn(t.Year(), t.Month(), t.Day()-7, hour, loc)
}

// nextSlot returns the earliest scheduled moment strictly after now.
func nextSlot(now time.Time, weekday time.Weekday, hour int, loc *time.Location) time.Time {
	t := now.In(loc)
	for i := 0; i <= 7; i++ {
		c := slotOn(t.Year(), t.Month(), t.Day()+i, hour, loc)
		if c.Weekday() == weekday && c.After(t) {
			return c
		}
	}
	return slotOn(t.Year(), t.Month(), t.Day()+7, hour, loc)
}

// periodRange returns the days analysed by a run at the given moment: the
// whole days before it, ending at its local midnight.
func periodRange(at time.Time, days int, loc *time.Location) (from, to time.Time) {
	t := at.In(loc)
	to = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	return to.AddDate(0, 0, -days), to
}

// monthStart returns the first moment of the UTC calendar month of t, the
// boundary of the monthly budget.
func monthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
