package database

import (
	"context"
	"evsys-back/entity"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// The slot claim is what keeps the weekly email from going out twice after a
// restart or with two instances; its atomicity is MongoDB's, so it is tested
// against MongoDB. Run as described in concurrency_test.go.

func TestClaimAIReportSlot(t *testing.T) {
	m := testMongo(t)
	ctx := context.Background()
	slot := time.Date(2026, 11, 2, 6, 0, 0, 0, time.UTC)

	claimed, err := m.ClaimAIReportSlot(ctx, slot)
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("no settings document, nothing to claim")
	}

	if err = m.SaveAIReportSettings(ctx, entity.DefaultAIReportSettings()); err != nil {
		t.Fatal(err)
	}

	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := m.ClaimAIReportSlot(ctx, slot)
			if err != nil {
				t.Error(err)
			}
			if ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("slot claimed %d times, want once", wins.Load())
	}

	// saving the settings from the admin page keeps the claim
	settings, err := m.GetAIReportSettings(ctx)
	if err != nil || settings == nil {
		t.Fatalf("settings: %v %v", settings, err)
	}
	settings.Hour = 9
	if err = m.SaveAIReportSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	if again, _ := m.ClaimAIReportSlot(ctx, slot); again {
		t.Error("an admin save must not reopen a claimed slot")
	}
	stored, _ := m.GetAIReportSettings(ctx)
	if stored.Hour != 9 || stored.LastScheduledRun == nil || !stored.LastScheduledRun.Equal(slot) {
		t.Errorf("stored %+v", stored)
	}

	if ok, _ := m.ClaimAIReportSlot(ctx, slot.AddDate(0, 0, 7)); !ok {
		t.Error("the next week's slot must be claimable")
	}
	if ok, _ := m.ClaimAIReportSlot(ctx, slot); ok {
		t.Error("an older slot must not be claimable")
	}

	// releasing an older slot leaves the newer claim alone
	next := slot.AddDate(0, 0, 7)
	if err = m.ReleaseAIReportSlot(ctx, slot); err != nil {
		t.Fatal(err)
	}
	if ok, _ := m.ClaimAIReportSlot(ctx, next); ok {
		t.Error("the release of another slot must not reopen this one")
	}
	if err = m.ReleaseAIReportSlot(ctx, next); err != nil {
		t.Fatal(err)
	}
	if ok, _ := m.ClaimAIReportSlot(ctx, next); !ok {
		t.Error("a released slot must be claimable again")
	}
}

func TestMailSubscriptionsByReportAndPeriod(t *testing.T) {
	m := testMongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	seed(t, m, collectionMailSubscriptions,
		// written before the report field existed
		bson.M{"_id": "old", "email": "old@example.com", "period": "weekly", "user_group": "default", "enabled": true, "created_at": now},
		bson.M{"_id": "stat", "email": "stat@example.com", "report": "statistic", "period": "weekly", "user_group": "default", "enabled": true, "created_at": now},
		bson.M{"_id": "ai", "email": "ai@example.com", "report": "ai_analysis", "period": "weekly", "language": "en", "enabled": true, "created_at": now},
	)

	weekly, err := m.ListMailSubscriptionsByPeriod(ctx, entity.MailPeriodWeekly)
	if err != nil {
		t.Fatal(err)
	}
	if ids := subIds(weekly); len(ids) != 2 || !ids["old"] || !ids["stat"] {
		t.Errorf("weekly statistic subscriptions = %v; the ai analysis must not get the statistic mail", ids)
	}

	ai, err := m.ListMailSubscriptionsByReport(ctx, entity.MailReportAIAnalysis)
	if err != nil {
		t.Fatal(err)
	}
	if ids := subIds(ai); len(ids) != 1 || !ids["ai"] || ai[0].Language != "en" {
		t.Errorf("ai subscriptions = %v", ids)
	}

	stat, err := m.ListMailSubscriptionsByReport(ctx, entity.MailReportStatistic)
	if err != nil {
		t.Fatal(err)
	}
	if ids := subIds(stat); len(ids) != 2 {
		t.Errorf("statistic subscriptions = %v", ids)
	}
}

func subIds(subs []*entity.MailSubscription) map[string]bool {
	ids := make(map[string]bool)
	for _, s := range subs {
		ids[s.Id] = true
	}
	return ids
}

func TestAIReportsAndUsage(t *testing.T) {
	m := testMongo(t)
	ctx := context.Background()
	if err := m.EnsureAIReportIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 11, 2, 6, 0, 0, 0, time.UTC)
	for i, id := range []string{"r1", "r2"} {
		r := &entity.AIReport{Id: id, Status: entity.AIReportRunning, Snapshot: `{"big":true}`, CreatedAt: base.Add(time.Duration(i) * time.Hour)}
		if err := m.SaveAIReport(ctx, r); err != nil {
			t.Fatal(err)
		}
		r.Status = entity.AIReportDone
		if err := m.SaveAIReport(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	list, err := m.ListAIReports(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Id != "r2" || list[0].Snapshot != "" || list[0].Status != entity.AIReportDone {
		t.Errorf("list = %+v", list)
	}
	full, _ := m.GetAIReport(ctx, "r1")
	if full == nil || full.Snapshot == "" {
		t.Error("a single report carries its snapshot")
	}
	if missing, err := m.GetAIReport(ctx, "nope"); missing != nil || err != nil {
		t.Errorf("missing report: %v %v", missing, err)
	}

	for i := 0; i < 3; i++ {
		u := &entity.AIUsage{Id: string(rune('a' + i)), Time: base.AddDate(0, 0, -i*20), CostUSD: 0.1}
		if err := m.SaveAIUsage(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	usage, err := m.ListAIUsage(ctx, base.AddDate(0, 0, -30), base)
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 2 || !usage[0].Time.Equal(base) {
		t.Errorf("usage = %+v", usage)
	}
}
