package entity

import (
	"sort"
	"time"
)

// A location's load is the sum of what its charge points draw at the same
// moment, so the questions an operator has about it - how often do sessions
// overlap, how much did the balancer promise them, how much did the site
// actually pull - are all questions about concurrency. This file holds the DTOs
// for that report and the sweep that produces it.
//
// Two peaks are reported and they mean different things. PeakAssignedAmps is
// the sum of the limits the load balancer handed out, which is a plan: it says
// what the site was allowed to draw. PeakPowerWatts is measured, from the meter
// values the chargers reported, and says what it did draw. A plan that is never
// approached says the limits are not binding; a draw that exceeds the plan says
// a limit was not in force. Neither number answers for the other, which is why
// both are here.

// ConcurrencySession is one charging session as it enters the sweep. The
// interval is already clamped to the report window.
type ConcurrencySession struct {
	TransactionId int    `json:"transaction_id" bson:"transaction_id"`
	ChargePointId string `json:"charge_point_id" bson:"charge_point_id"`
	ConnectorId   int    `json:"connector_id" bson:"connector_id"`
	// PowerLimit is the amperage the load balancer assigned, in amps. Zero
	// means no limit was recorded - an unbalanced charge point, or a session
	// that predates the balancer.
	PowerLimit int       `json:"power_limit" bson:"power_limit"`
	TimeStart  time.Time `json:"time_start" bson:"time_start"`
	TimeStop   time.Time `json:"time_stop" bson:"time_stop"`

	// IsFinished and LocationId steer the sweep and are not part of the
	// response: a live session has no stop time to clamp against, and the
	// location is already the key the rows are grouped under.
	IsFinished bool   `json:"-" bson:"is_finished"`
	LocationId string `json:"-" bson:"location_id"`
}

// SegmentSession identifies one session inside a segment. It repeats the limit
// rather than referring back to the session list so a segment can be read on
// its own.
type SegmentSession struct {
	TransactionId int    `json:"transaction_id"`
	ChargePointId string `json:"charge_point_id"`
	ConnectorId   int    `json:"connector_id"`
	PowerLimit    int    `json:"power_limit"`
}

// ConcurrencySegment is a stretch of time over which the set of charging
// sessions did not change. Segment boundaries are session starts and stops, so
// a segment is the longest interval that can be described by a single answer to
// "who is charging and under what limit".
type ConcurrencySegment struct {
	From    time.Time `json:"from"`
	To      time.Time `json:"to"`
	Seconds int64     `json:"seconds"`
	// Sessions is len(Detail), repeated so a caller can read the shape of the
	// segment without walking the list.
	Sessions int `json:"sessions"`
	// AssignedAmps sums PowerLimit over the segment's sessions. Sessions with no
	// recorded limit contribute zero, so this understates rather than invents.
	AssignedAmps int              `json:"assigned_amps"`
	Detail       []SegmentSession `json:"detail"`
}

// ConcurrencyLevel is how long a location spent with exactly this many sessions
// charging. Level 0 is included: idle time is the context that makes the rest
// of the report legible.
type ConcurrencyLevel struct {
	Sessions int   `json:"sessions"`
	Seconds  int64 `json:"seconds"`
}

// SiteConcurrency is the report for one location over the requested window.
type SiteConcurrency struct {
	LocationId   string `json:"location_id"`
	LocationName string `json:"location_name,omitempty"`

	From time.Time `json:"from"`
	To   time.Time `json:"to"`

	// Sessions counts the sessions that overlapped the window at all, including
	// those that started before it or ran past its end.
	Sessions int `json:"sessions"`

	// MaxSessions is the highest number charging at once, and OverlapSeconds
	// the time at least two were.
	MaxSessions    int   `json:"max_sessions"`
	OverlapSeconds int64 `json:"overlap_seconds"`

	// PeakAssignedAmps is the highest sum of assigned limits over any segment -
	// what the balancer permitted, not what was drawn. PeakAssignedAt is the
	// start of the segment where it happened.
	PeakAssignedAmps int       `json:"peak_assigned_amps"`
	PeakAssignedAt   time.Time `json:"peak_assigned_at,omitempty"`

	// PeakPowerWatts is the highest concurrent draw the site actually supplied,
	// measured from reported meter values and resolved to the minute.
	// PeakPowerSessions is how many sessions were drawing in that minute.
	//
	// It is zero when no session in the window reported power. That is not the
	// same as an idle site: chargers that do not report MeterValues, or report
	// them without a power measurand, are invisible here while still drawing.
	PeakPowerWatts    float64   `json:"peak_power_watts"`
	PeakPowerAt       time.Time `json:"peak_power_at,omitempty"`
	PeakPowerSessions int       `json:"peak_power_sessions"`

	// Levels is the time spent at each concurrency count, ascending from 0.
	Levels []ConcurrencyLevel `json:"levels"`

	// Segments are those meeting the requested minimum session count, in time
	// order. Truncated is set when the cap cut the list short; the summary
	// fields above are computed before any truncation and stay exact.
	Segments  []ConcurrencySegment `json:"segments"`
	Truncated bool                 `json:"truncated,omitempty"`
}

// SitePeakPower is the measured peak for one location, produced by the
// aggregation and folded into SiteConcurrency.
type SitePeakPower struct {
	LocationId string    `json:"location_id" bson:"_id"`
	Watts      float64   `json:"watts" bson:"watts"`
	At         time.Time `json:"at" bson:"at"`
	Sessions   int       `json:"sessions" bson:"sessions"`
}

// BuildSiteConcurrency runs the sweep for one location.
//
// Sessions are clamped to [from, to] first: one that started before the window
// contributes only the part inside it, and a session still running has no stop
// time at all, so the window's end stands in for one. Boundaries are then the
// distinct clamped start and stop instants, and between two consecutive
// boundaries the active set cannot change - which is what makes a segment the
// unit the whole report is built from.
//
// minSessions filters which segments are returned, not which are measured: the
// levels and the peaks are computed over every segment, so asking only for
// overlaps does not change the numbers above them. maxSegments caps the
// returned list; zero or less means no cap.
func BuildSiteConcurrency(sessions []ConcurrencySession, from, to time.Time, minSessions, maxSegments int) SiteConcurrency {
	report := SiteConcurrency{From: from, To: to, Segments: []ConcurrencySegment{}, Levels: []ConcurrencyLevel{}}
	if !to.After(from) {
		return report
	}

	type span struct {
		s     ConcurrencySession
		start time.Time
		stop  time.Time
	}
	spans := make([]span, 0, len(sessions))
	for _, s := range sessions {
		start, stop := s.TimeStart, s.TimeStop
		// A live session, or one whose stop was never written, runs to the end
		// of the window rather than to the zero time - which would otherwise
		// sort before every start and drop the session silently.
		if !s.IsFinished || stop.IsZero() {
			stop = to
		}
		if start.Before(from) {
			start = from
		}
		if stop.After(to) {
			stop = to
		}
		if !stop.After(start) {
			continue
		}
		spans = append(spans, span{s: s, start: start, stop: stop})
	}
	report.Sessions = len(spans)
	if len(spans) == 0 {
		report.Levels = []ConcurrencyLevel{{Sessions: 0, Seconds: int64(to.Sub(from).Seconds())}}
		return report
	}

	// The distinct instants at which the active set can change. Ties collapse
	// here, so several sessions starting in the same second produce one
	// boundary and never a zero-length segment between them.
	seen := make(map[int64]struct{}, len(spans)*2)
	bounds := make([]time.Time, 0, len(spans)*2+2)
	add := func(t time.Time) {
		if t.Before(from) || t.After(to) {
			return
		}
		if _, ok := seen[t.UnixNano()]; ok {
			return
		}
		seen[t.UnixNano()] = struct{}{}
		bounds = append(bounds, t)
	}
	add(from)
	add(to)
	for _, sp := range spans {
		add(sp.start)
		add(sp.stop)
	}
	sort.Slice(bounds, func(i, j int) bool { return bounds[i].Before(bounds[j]) })

	levels := make(map[int]int64)
	var all []ConcurrencySegment
	for i := 0; i+1 < len(bounds); i++ {
		segFrom, segTo := bounds[i], bounds[i+1]
		var detail []SegmentSession
		amps := 0
		for _, sp := range spans {
			// half-open: a session that stops exactly when another starts does
			// not overlap it
			if !sp.start.After(segFrom) && sp.stop.After(segFrom) {
				detail = append(detail, SegmentSession{
					TransactionId: sp.s.TransactionId,
					ChargePointId: sp.s.ChargePointId,
					ConnectorId:   sp.s.ConnectorId,
					PowerLimit:    sp.s.PowerLimit,
				})
				amps += sp.s.PowerLimit
			}
		}
		seconds := int64(segTo.Sub(segFrom).Seconds())
		levels[len(detail)] += seconds

		if len(detail) > report.MaxSessions {
			report.MaxSessions = len(detail)
		}
		if len(detail) >= 2 {
			report.OverlapSeconds += seconds
		}
		if amps > report.PeakAssignedAmps {
			report.PeakAssignedAmps = amps
			report.PeakAssignedAt = segFrom
		}
		if len(detail) == 0 {
			continue
		}
		sort.Slice(detail, func(a, b int) bool { return detail[a].TransactionId < detail[b].TransactionId })
		all = append(all, ConcurrencySegment{
			From: segFrom, To: segTo, Seconds: seconds,
			Sessions: len(detail), AssignedAmps: amps, Detail: detail,
		})
	}

	counts := make([]int, 0, len(levels))
	for n := range levels {
		counts = append(counts, n)
	}
	sort.Ints(counts)
	for _, n := range counts {
		report.Levels = append(report.Levels, ConcurrencyLevel{Sessions: n, Seconds: levels[n]})
	}

	for _, seg := range all {
		if seg.Sessions < minSessions {
			continue
		}
		if maxSegments > 0 && len(report.Segments) >= maxSegments {
			report.Truncated = true
			break
		}
		report.Segments = append(report.Segments, seg)
	}
	return report
}
