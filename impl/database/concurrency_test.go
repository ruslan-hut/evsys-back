package database

import (
	"context"
	"evsys-back/entity"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Integration tests for the site concurrency report. The sweep itself is unit
// tested in entity; what is tested here is the part that cannot be - the
// aggregation. Its per-minute collapse, its location join and its window
// bounds are all pipeline behaviour, and a mock would only re-assert the
// pipeline I wrote rather than what MongoDB does with it.
//
//	docker run -d --name evsys-test-mongo -p 27019:27017 mongo:7
//	MONGO_TEST_URI=mongodb://localhost:27019 go test ./impl/database/...

const concurrencyTestDatabase = "evsys_back_concurrency_test"

func testMongo(t *testing.T) *MongoDB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI is not set")
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err = client.Ping(ctx, nil); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if err = client.Database(concurrencyTestDatabase).Drop(ctx); err != nil {
		t.Fatalf("drop database: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(ctx) })

	return &MongoDB{client: client, database: concurrencyTestDatabase}
}

func seed(t *testing.T, m *MongoDB, col string, docs ...any) {
	t.Helper()
	if len(docs) == 0 {
		return
	}
	if _, err := m.col(col).InsertMany(context.Background(), docs); err != nil {
		t.Fatalf("seed %s: %v", col, err)
	}
}

var day = time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

func mm(min int) time.Time { return day.Add(time.Duration(min) * time.Minute) }

func sample(watts int, at time.Time) bson.M {
	return bson.M{"power_rate": watts, "time": at}
}

// tx builds a finished transaction document in the shape evsys writes.
func tx(id int, cp string, connector, limit, startMin, stopMin int, meters ...bson.M) bson.M {
	if meters == nil {
		meters = []bson.M{}
	}
	return bson.M{
		"transaction_id":  id,
		"charge_point_id": cp,
		"connector_id":    connector,
		"power_limit":     limit,
		"is_finished":     true,
		"time_start":      mm(startMin),
		"time_stop":       mm(stopMin),
		"meter_start":     0,
		"meter_stop":      1000,
		"meter_values":    meters,
	}
}

func seedSite(t *testing.T, m *MongoDB) {
	t.Helper()
	seed(t, m, collectionLocations,
		bson.M{"id": "loc1", "name": "Main site"},
		bson.M{"id": "loc2", "name": "Office"},
	)
	seed(t, m, collectionChargePoints,
		bson.M{"charge_point_id": "PE00001", "location_id": "loc1"},
		bson.M{"charge_point_id": "PE00002", "location_id": "loc1"},
		bson.M{"charge_point_id": "PE00003", "location_id": "loc1"},
		bson.M{"charge_point_id": "WB1", "location_id": "loc2"},
		// a charge point with no location cannot be attributed to a supply
		bson.M{"charge_point_id": "ORPHAN"},
	)
}

func find(t *testing.T, rows []*entity.SiteConcurrency, loc string) *entity.SiteConcurrency {
	t.Helper()
	for _, r := range rows {
		if r.LocationId == loc {
			return r
		}
	}
	t.Fatalf("no row for location %q", loc)
	return nil
}

// The shape from the log the report was written for: three sessions overlapping
// at one site while an unrelated site runs its own, and the two must not be
// added together.
func TestSiteConcurrencyGroupsByLocation(t *testing.T) {
	m := testMongo(t)
	seedSite(t, m)
	seed(t, m, collectionTransactions,
		tx(1, "PE00001", 1, 150, 0, 40),
		tx(2, "PE00002", 2, 115, 10, 30),
		tx(3, "PE00003", 1, 85, 20, 50),
		tx(4, "WB1", 1, 0, 5, 55),
	)

	rows, err := m.SiteConcurrency(context.Background(), mm(0), mm(60), "", 2, 0)
	if err != nil {
		t.Fatalf("SiteConcurrency: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d locations, want 2", len(rows))
	}

	main := find(t, rows, "loc1")
	if main.LocationName != "Main site" {
		t.Errorf("location name = %q, want %q", main.LocationName, "Main site")
	}
	if main.Sessions != 3 || main.MaxSessions != 3 {
		t.Errorf("loc1: %d sessions, max %d, want 3 and 3", main.Sessions, main.MaxSessions)
	}
	if main.PeakAssignedAmps != 350 {
		t.Errorf("loc1 peak assigned = %d A, want 350", main.PeakAssignedAmps)
	}

	office := find(t, rows, "loc2")
	if office.MaxSessions != 1 {
		t.Errorf("loc2 max sessions = %d, want 1: sites do not share a supply", office.MaxSessions)
	}
}

// A charge point with no location has no site to load, and bundling those under
// an empty key would report a peak nobody supplies.
func TestSiteConcurrencyExcludesChargePointWithoutLocation(t *testing.T) {
	m := testMongo(t)
	seedSite(t, m)
	seed(t, m, collectionTransactions,
		tx(1, "ORPHAN", 1, 150, 0, 40, sample(40000, mm(10))),
		tx(2, "PE00001", 1, 150, 0, 40),
	)

	rows, err := m.SiteConcurrency(context.Background(), mm(0), mm(60), "", 2, 0)
	if err != nil {
		t.Fatalf("SiteConcurrency: %v", err)
	}
	for _, r := range rows {
		if r.LocationId == "" {
			t.Fatalf("a charge point with no location produced a site row: %+v", r)
		}
	}
	if len(rows) != 1 || rows[0].LocationId != "loc1" {
		t.Fatalf("got %d rows, want only loc1", len(rows))
	}
}

// The measured peak is the sum across sessions in the same minute, and the
// minute is where a charger reporting more than one sample would otherwise be
// counted repeatedly.
func TestSiteConcurrencyPeakPowerSumsSessionsAndCollapsesDuplicateSamples(t *testing.T) {
	m := testMongo(t)
	seedSite(t, m)
	seed(t, m, collectionTransactions,
		// two samples inside minute 30: only the larger counts for this session
		tx(1, "PE00001", 1, 150, 0, 40,
			sample(30000, mm(30).Add(10*time.Second)),
			sample(31000, mm(30).Add(40*time.Second)),
			sample(5000, mm(35)),
		),
		// a second session drawing in the same minute
		tx(2, "PE00002", 2, 115, 10, 40,
			sample(20000, mm(30).Add(20*time.Second)),
			sample(1000, mm(35)),
		),
	)

	rows, err := m.SiteConcurrency(context.Background(), mm(0), mm(60), "", 2, 0)
	if err != nil {
		t.Fatalf("SiteConcurrency: %v", err)
	}
	main := find(t, rows, "loc1")

	if main.PeakPowerWatts != 51000 {
		t.Fatalf("peak power = %.0f W, want 51000: 31000 + 20000, the duplicate sample counted once",
			main.PeakPowerWatts)
	}
	if main.PeakPowerAt == nil || !main.PeakPowerAt.Equal(mm(30)) {
		t.Errorf("peak at %v, want the minute %v", main.PeakPowerAt, mm(30))
	}
	if main.PeakPowerSessions != 2 {
		t.Errorf("peak sessions = %d, want 2", main.PeakPowerSessions)
	}
}

// Samples outside the window belong to another report. A session that spans the
// boundary keeps only the part inside it.
func TestSiteConcurrencyPeakPowerIgnoresSamplesOutsideWindow(t *testing.T) {
	m := testMongo(t)
	seedSite(t, m)
	seed(t, m, collectionTransactions,
		tx(1, "PE00001", 1, 150, -120, 40,
			sample(90000, mm(-60)), // before the window
			sample(20000, mm(10)),  // inside
			sample(80000, mm(120)), // after
		),
	)

	rows, err := m.SiteConcurrency(context.Background(), mm(0), mm(60), "", 1, 0)
	if err != nil {
		t.Fatalf("SiteConcurrency: %v", err)
	}
	main := find(t, rows, "loc1")
	if main.PeakPowerWatts != 20000 {
		t.Fatalf("peak power = %.0f W, want 20000: samples outside the window were counted",
			main.PeakPowerWatts)
	}
}

// A session running when the window opened is part of the site's load. Matching
// on time_stop alone, as the older power reports do, would drop it.
func TestSiteConcurrencyIncludesSessionsSpanningTheWindow(t *testing.T) {
	m := testMongo(t)
	seedSite(t, m)
	seed(t, m, collectionTransactions,
		// started before the window and still running when it closed
		tx(1, "PE00001", 1, 150, -60, 120, sample(40000, mm(30))),
		tx(2, "PE00002", 2, 115, 10, 50, sample(10000, mm(30))),
	)

	rows, err := m.SiteConcurrency(context.Background(), mm(0), mm(60), "", 2, 0)
	if err != nil {
		t.Fatalf("SiteConcurrency: %v", err)
	}
	main := find(t, rows, "loc1")
	if main.Sessions != 2 || main.MaxSessions != 2 {
		t.Fatalf("%d sessions, max %d, want 2 and 2: the spanning session was dropped",
			main.Sessions, main.MaxSessions)
	}
	if main.PeakAssignedAmps != 265 {
		t.Errorf("peak assigned = %d A, want 265", main.PeakAssignedAmps)
	}
	if main.PeakPowerWatts != 50000 {
		t.Errorf("peak power = %.0f W, want 50000", main.PeakPowerWatts)
	}
	// clamped: 40 minutes of overlap, not the session's full length
	if main.OverlapSeconds != 40*60 {
		t.Errorf("overlap = %d s, want %d", main.OverlapSeconds, 40*60)
	}
}

// A live session has no time_stop. It is still drawing, so it has to reach the
// report rather than be filtered out by a stop-time match.
func TestSiteConcurrencyIncludesUnfinishedSessions(t *testing.T) {
	m := testMongo(t)
	seedSite(t, m)
	unfinished := tx(1, "PE00001", 1, 150, 10, 0)
	unfinished["is_finished"] = false
	delete(unfinished, "time_stop")
	seed(t, m, collectionTransactions,
		unfinished,
		tx(2, "PE00002", 2, 115, 20, 50),
	)

	rows, err := m.SiteConcurrency(context.Background(), mm(0), mm(60), "", 2, 0)
	if err != nil {
		t.Fatalf("SiteConcurrency: %v", err)
	}
	main := find(t, rows, "loc1")
	if main.Sessions != 2 {
		t.Fatalf("sessions = %d, want 2: the live session was dropped", main.Sessions)
	}
	if main.MaxSessions != 2 || main.PeakAssignedAmps != 265 {
		t.Errorf("max %d sessions at %d A, want 2 at 265", main.MaxSessions, main.PeakAssignedAmps)
	}
}

func TestSiteConcurrencyFiltersByLocation(t *testing.T) {
	m := testMongo(t)
	seedSite(t, m)
	seed(t, m, collectionTransactions,
		tx(1, "PE00001", 1, 150, 0, 40, sample(40000, mm(10))),
		tx(2, "WB1", 1, 100, 0, 40, sample(90000, mm(10))),
	)

	rows, err := m.SiteConcurrency(context.Background(), mm(0), mm(60), "loc2", 1, 0)
	if err != nil {
		t.Fatalf("SiteConcurrency: %v", err)
	}
	if len(rows) != 1 || rows[0].LocationId != "loc2" {
		t.Fatalf("got %d rows, want only loc2: %+v", len(rows), rows)
	}
	if rows[0].PeakPowerWatts != 90000 {
		t.Errorf("peak power = %.0f W, want 90000: the other site leaked in", rows[0].PeakPowerWatts)
	}
}

// Sessions that never overlap still make a site row, because the measured peak
// and the idle time are worth reporting on their own.
func TestSiteConcurrencyReportsSiteWithNoOverlap(t *testing.T) {
	m := testMongo(t)
	seedSite(t, m)
	seed(t, m, collectionTransactions,
		tx(1, "PE00001", 1, 150, 0, 20, sample(40000, mm(10))),
		tx(2, "PE00002", 2, 115, 30, 50, sample(25000, mm(40))),
	)

	rows, err := m.SiteConcurrency(context.Background(), mm(0), mm(60), "", 2, 0)
	if err != nil {
		t.Fatalf("SiteConcurrency: %v", err)
	}
	main := find(t, rows, "loc1")
	if main.MaxSessions != 1 || main.OverlapSeconds != 0 {
		t.Errorf("max %d sessions, %d s overlap, want 1 and 0", main.MaxSessions, main.OverlapSeconds)
	}
	if len(main.Segments) != 0 {
		t.Errorf("got %d segments at min_sessions=2, want none", len(main.Segments))
	}
	if main.PeakPowerWatts != 40000 {
		t.Errorf("peak power = %.0f W, want 40000", main.PeakPowerWatts)
	}
}

// The peak carries the minute it happened in, which is what makes it traceable
// back to a session. A $max would give the right watts attached to the wrong
// moment, so the peak here is deliberately not the first minute with data.
func TestSiteConcurrencyPeakPowerReportsTheMinuteOfThePeak(t *testing.T) {
	m := testMongo(t)
	seedSite(t, m)
	seed(t, m, collectionTransactions,
		tx(1, "PE00001", 1, 150, 0, 50,
			sample(5000, mm(5)),   // earliest, but small
			sample(45000, mm(40)), // the peak, late in the window
			sample(9000, mm(45)),
		),
	)

	rows, err := m.SiteConcurrency(context.Background(), mm(0), mm(60), "", 1, 0)
	if err != nil {
		t.Fatalf("SiteConcurrency: %v", err)
	}
	main := find(t, rows, "loc1")

	if main.PeakPowerWatts != 45000 {
		t.Fatalf("peak power = %.0f W, want 45000", main.PeakPowerWatts)
	}
	if main.PeakPowerAt == nil || !main.PeakPowerAt.Equal(mm(40)) {
		t.Fatalf("peak at %v, want %v: the peak's minute was not carried with its value",
			main.PeakPowerAt, mm(40))
	}
}
