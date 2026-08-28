package entity

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

var base = time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

func at(min int) time.Time { return base.Add(time.Duration(min) * time.Minute) }

func session(id, limit, startMin, stopMin int) ConcurrencySession {
	return ConcurrencySession{
		TransactionId: id,
		ChargePointId: "CP1",
		ConnectorId:   1,
		PowerLimit:    limit,
		TimeStart:     at(startMin),
		TimeStop:      at(stopMin),
		IsFinished:    true,
	}
}

// segmentAt finds the segment covering a minute offset, so a test can assert on
// a moment rather than on a list index that shifts when the fixture changes.
func segmentAt(r SiteConcurrency, min int) *ConcurrencySegment {
	t := at(min)
	for i := range r.Segments {
		if !r.Segments[i].From.After(t) && r.Segments[i].To.After(t) {
			return &r.Segments[i]
		}
	}
	return nil
}

func levelSeconds(r SiteConcurrency, n int) int64 {
	for _, l := range r.Levels {
		if l.Sessions == n {
			return l.Seconds
		}
	}
	return 0
}

func TestBuildSiteConcurrencyNoSessions(t *testing.T) {
	r := BuildSiteConcurrency(nil, at(0), at(60), 2, 0)
	if r.Sessions != 0 || r.MaxSessions != 0 || r.OverlapSeconds != 0 {
		t.Fatalf("expected an empty report, got %+v", r)
	}
	if got := levelSeconds(r, 0); got != 3600 {
		t.Fatalf("idle seconds = %d, want the whole window, 3600", got)
	}
	if r.Segments == nil {
		t.Error("segments must marshal as [] rather than null")
	}
}

// A single session is not an overlap, but it is still load: the levels and the
// assigned peak have to see it even though no segment is returned.
func TestBuildSiteConcurrencySingleSession(t *testing.T) {
	r := BuildSiteConcurrency([]ConcurrencySession{session(1, 150, 10, 40)}, at(0), at(60), 2, 0)

	if r.MaxSessions != 1 {
		t.Errorf("max sessions = %d, want 1", r.MaxSessions)
	}
	if r.OverlapSeconds != 0 {
		t.Errorf("overlap seconds = %d, want 0", r.OverlapSeconds)
	}
	if r.PeakAssignedAmps != 150 {
		t.Errorf("peak assigned = %d A, want 150", r.PeakAssignedAmps)
	}
	if r.PeakAssignedAt == nil || !r.PeakAssignedAt.Equal(at(10)) {
		t.Errorf("peak assigned at %v, want %v", r.PeakAssignedAt, at(10))
	}
	if len(r.Segments) != 0 {
		t.Errorf("min_sessions=2 returned %d segments", len(r.Segments))
	}
	if got := levelSeconds(r, 1); got != 1800 {
		t.Errorf("one-session seconds = %d, want 1800", got)
	}
	if got := levelSeconds(r, 0); got != 1800 {
		t.Errorf("idle seconds = %d, want 1800", got)
	}
}

// The shape the report exists for: sessions arriving and leaving while others
// keep charging, so the site total steps up and down.
func TestBuildSiteConcurrencyOverlapSteps(t *testing.T) {
	r := BuildSiteConcurrency([]ConcurrencySession{
		session(1, 150, 0, 40),
		session(2, 115, 10, 30),
		session(3, 85, 20, 50),
	}, at(0), at(60), 2, 0)

	if r.MaxSessions != 3 {
		t.Fatalf("max sessions = %d, want 3", r.MaxSessions)
	}
	if r.PeakAssignedAmps != 350 {
		t.Errorf("peak assigned = %d A, want 350", r.PeakAssignedAmps)
	}
	if r.PeakAssignedAt == nil || !r.PeakAssignedAt.Equal(at(20)) {
		t.Errorf("peak assigned at %v, want %v", r.PeakAssignedAt, at(20))
	}
	// 10-30 two, 20-30 three, 30-40 two, 40-50 one: overlap runs 10..40
	if r.OverlapSeconds != 30*60 {
		t.Errorf("overlap seconds = %d, want %d", r.OverlapSeconds, 30*60)
	}
	if got := levelSeconds(r, 3); got != 10*60 {
		t.Errorf("three-session seconds = %d, want %d", got, 10*60)
	}

	if seg := segmentAt(r, 25); seg == nil || seg.Sessions != 3 || seg.AssignedAmps != 350 {
		t.Fatalf("segment at +25 = %+v, want 3 sessions at 350 A", seg)
	}
	if seg := segmentAt(r, 35); seg == nil || seg.Sessions != 2 || seg.AssignedAmps != 235 {
		t.Fatalf("segment at +35 = %+v, want 2 sessions at 235 A", seg)
	}
	// the tail with only #3 left is below min_sessions and must not be returned
	if seg := segmentAt(r, 45); seg != nil {
		t.Fatalf("segment at +45 = %+v, want none below min_sessions", seg)
	}
}

// Adjacent sessions are not concurrent. Treating the interval as closed at both
// ends would invent an overlap at every handover and inflate the peak.
func TestBuildSiteConcurrencyTouchingSessionsDoNotOverlap(t *testing.T) {
	r := BuildSiteConcurrency([]ConcurrencySession{
		session(1, 150, 0, 30),
		session(2, 150, 30, 60),
	}, at(0), at(60), 2, 0)

	if r.MaxSessions != 1 {
		t.Errorf("max sessions = %d, want 1: one stops exactly as the other starts", r.MaxSessions)
	}
	if r.OverlapSeconds != 0 {
		t.Errorf("overlap seconds = %d, want 0", r.OverlapSeconds)
	}
	if r.PeakAssignedAmps != 150 {
		t.Errorf("peak assigned = %d A, want 150, not both limits summed", r.PeakAssignedAmps)
	}
}

// Several sessions starting in the same second are one boundary, not several.
func TestBuildSiteConcurrencySimultaneousStarts(t *testing.T) {
	r := BuildSiteConcurrency([]ConcurrencySession{
		session(1, 150, 10, 40),
		session(2, 115, 10, 40),
	}, at(0), at(60), 2, 0)

	if len(r.Segments) != 1 {
		t.Fatalf("got %d segments, want 1: identical intervals cannot produce a zero-length segment", len(r.Segments))
	}
	if r.Segments[0].Sessions != 2 || r.Segments[0].AssignedAmps != 265 {
		t.Errorf("segment = %+v, want 2 sessions at 265 A", r.Segments[0])
	}
	if r.Segments[0].Seconds != 30*60 {
		t.Errorf("segment seconds = %d, want %d", r.Segments[0].Seconds, 30*60)
	}
}

// One session wholly inside another.
func TestBuildSiteConcurrencyNestedSession(t *testing.T) {
	r := BuildSiteConcurrency([]ConcurrencySession{
		session(1, 150, 0, 60),
		session(2, 85, 20, 30),
	}, at(0), at(60), 2, 0)

	if r.MaxSessions != 2 || r.PeakAssignedAmps != 235 {
		t.Fatalf("max %d sessions at %d A, want 2 at 235", r.MaxSessions, r.PeakAssignedAmps)
	}
	if r.OverlapSeconds != 10*60 {
		t.Errorf("overlap seconds = %d, want %d", r.OverlapSeconds, 10*60)
	}
}

// A session that began before the window or runs past its end contributes only
// the part inside it, or every report would count time it does not cover.
func TestBuildSiteConcurrencyClampsToWindow(t *testing.T) {
	r := BuildSiteConcurrency([]ConcurrencySession{
		{TransactionId: 1, PowerLimit: 150, TimeStart: at(-120), TimeStop: at(30), IsFinished: true},
		{TransactionId: 2, PowerLimit: 115, TimeStart: at(20), TimeStop: at(600), IsFinished: true},
	}, at(0), at(60), 2, 0)

	if got := levelSeconds(r, 1); got != 50*60 {
		t.Errorf("one-session seconds = %d, want %d: 0-20 plus 30-60", got, 50*60)
	}
	if r.OverlapSeconds != 10*60 {
		t.Errorf("overlap seconds = %d, want %d", r.OverlapSeconds, 10*60)
	}
	if got := levelSeconds(r, 0); got != 0 {
		t.Errorf("idle seconds = %d, want 0: the window is covered end to end", got)
	}
}

// A live session has no stop time. Left as the zero time it sorts before every
// start and vanishes from the report while its car is still drawing.
func TestBuildSiteConcurrencyUnfinishedSessionRunsToWindowEnd(t *testing.T) {
	r := BuildSiteConcurrency([]ConcurrencySession{
		{TransactionId: 1, PowerLimit: 150, TimeStart: at(10), IsFinished: false},
		session(2, 115, 20, 40),
	}, at(0), at(60), 2, 0)

	if r.Sessions != 2 {
		t.Fatalf("sessions = %d, want 2: the live one was dropped", r.Sessions)
	}
	if got := levelSeconds(r, 1); got != 30*60 {
		t.Errorf("one-session seconds = %d, want %d: 10-20 plus 40-60", got, 30*60)
	}
	if r.OverlapSeconds != 20*60 {
		t.Errorf("overlap seconds = %d, want %d", r.OverlapSeconds, 20*60)
	}
}

// A session with no recorded limit still occupies the site; it just adds nothing
// to the assigned total, which must not become a reason to omit it.
func TestBuildSiteConcurrencyUnlimitedSessionCountsButAddsNoAmps(t *testing.T) {
	r := BuildSiteConcurrency([]ConcurrencySession{
		session(1, 150, 0, 60),
		session(2, 0, 10, 50),
	}, at(0), at(60), 2, 0)

	if r.MaxSessions != 2 {
		t.Fatalf("max sessions = %d, want 2", r.MaxSessions)
	}
	if r.PeakAssignedAmps != 150 {
		t.Errorf("peak assigned = %d A, want 150: an unrecorded limit contributes nothing", r.PeakAssignedAmps)
	}
	if seg := segmentAt(r, 20); seg == nil || seg.Sessions != 2 {
		t.Fatalf("segment at +20 = %+v, want both sessions", seg)
	}
}

// min_sessions selects what is returned, not what is measured.
func TestBuildSiteConcurrencyMinSessionsDoesNotChangeSummary(t *testing.T) {
	in := []ConcurrencySession{
		session(1, 150, 0, 40),
		session(2, 115, 10, 30),
	}
	all := BuildSiteConcurrency(in, at(0), at(60), 1, 0)
	overlaps := BuildSiteConcurrency(in, at(0), at(60), 2, 0)

	if all.PeakAssignedAmps != overlaps.PeakAssignedAmps ||
		all.OverlapSeconds != overlaps.OverlapSeconds ||
		all.MaxSessions != overlaps.MaxSessions {
		t.Fatalf("summary moved with min_sessions: %+v vs %+v", all, overlaps)
	}
	if len(all.Segments) <= len(overlaps.Segments) {
		t.Errorf("min_sessions=1 returned %d segments, min_sessions=2 returned %d",
			len(all.Segments), len(overlaps.Segments))
	}
}

// The cap bounds the response, and says so, without disturbing the totals.
func TestBuildSiteConcurrencyTruncationKeepsSummaryExact(t *testing.T) {
	var in []ConcurrencySession
	for i := 0; i < 8; i++ {
		in = append(in, session(i*2+1, 150, i*5, i*5+8))
		in = append(in, session(i*2+2, 85, i*5+1, i*5+7))
	}
	full := BuildSiteConcurrency(in, at(0), at(120), 2, 0)
	capped := BuildSiteConcurrency(in, at(0), at(120), 2, 3)

	if capped.Truncated != true {
		t.Fatal("truncated flag not set")
	}
	if len(capped.Segments) != 3 {
		t.Fatalf("returned %d segments, want the cap of 3", len(capped.Segments))
	}
	if full.Truncated {
		t.Error("uncapped report reported truncation")
	}
	if capped.PeakAssignedAmps != full.PeakAssignedAmps || capped.OverlapSeconds != full.OverlapSeconds {
		t.Errorf("truncation changed the summary: %d A / %d s vs %d A / %d s",
			capped.PeakAssignedAmps, capped.OverlapSeconds,
			full.PeakAssignedAmps, full.OverlapSeconds)
	}
}

func TestBuildSiteConcurrencyRejectsInvertedWindow(t *testing.T) {
	r := BuildSiteConcurrency([]ConcurrencySession{session(1, 150, 0, 10)}, at(60), at(0), 2, 0)
	if r.Sessions != 0 || len(r.Segments) != 0 {
		t.Fatalf("an inverted window produced a report: %+v", r)
	}
}

// Segments are consumed as a timeline, so they have to arrive in time order and
// leave no gap where the site was busy.
func TestBuildSiteConcurrencySegmentsAreContiguousAndOrdered(t *testing.T) {
	r := BuildSiteConcurrency([]ConcurrencySession{
		session(1, 150, 0, 40),
		session(2, 115, 10, 30),
		session(3, 85, 20, 50),
	}, at(0), at(60), 1, 0)

	if len(r.Segments) < 2 {
		t.Fatalf("got %d segments, want several", len(r.Segments))
	}
	for i, seg := range r.Segments {
		if !seg.To.After(seg.From) {
			t.Fatalf("segment %d is empty: %v..%v", i, seg.From, seg.To)
		}
		if i > 0 && !r.Segments[i-1].To.Equal(seg.From) {
			t.Fatalf("gap between segment %d (ends %v) and %d (starts %v)",
				i-1, r.Segments[i-1].To, i, seg.From)
		}
	}
}

// encoding/json's omitempty does not apply to a struct, so a bare time.Time
// here reached clients as 0001-01-01T00:00:00Z and rendered as a real date -
// "31 Dec 23:45" on a site whose sessions carried no assigned limit.
func TestBuildSiteConcurrencyOmitsPeakTimeWhenThereIsNoPeak(t *testing.T) {
	r := BuildSiteConcurrency([]ConcurrencySession{
		{TransactionId: 1, PowerLimit: 0, TimeStart: at(10), TimeStop: at(40), IsFinished: true},
	}, at(0), at(60), 2, 0)

	if r.PeakAssignedAmps != 0 {
		t.Fatalf("peak assigned = %d A, want 0", r.PeakAssignedAmps)
	}
	if r.PeakAssignedAt != nil {
		t.Errorf("peak assigned at %v, want nil so the field is omitted", r.PeakAssignedAt)
	}

	blob, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if bytes.Contains(blob, []byte("peak_assigned_at")) {
		t.Errorf("peak_assigned_at was serialised with no peak to report: %s", blob)
	}
	if bytes.Contains(blob, []byte("0001-01-01")) {
		t.Errorf("the zero time reached the response: %s", blob)
	}
}
