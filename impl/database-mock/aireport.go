package database_mock

import (
	"context"
	"evsys-back/entity"
	"sort"
	"time"
)

func (db *MockDB) GetAIReportSettings(_ context.Context) (*entity.AIReportSettings, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	if db.aiSettings == nil {
		return nil, nil
	}
	s := *db.aiSettings
	return &s, nil
}

func (db *MockDB) SaveAIReportSettings(_ context.Context, settings *entity.AIReportSettings) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	s := *settings
	if db.aiSettings != nil {
		s.LastScheduledRun = db.aiSettings.LastScheduledRun
	} else {
		s.LastScheduledRun = nil
	}
	db.aiSettings = &s
	return nil
}

func (db *MockDB) ClaimAIReportSlot(_ context.Context, slot time.Time) (bool, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	if db.aiSettings == nil {
		return false, nil
	}
	if last := db.aiSettings.LastScheduledRun; last != nil && !last.Before(slot) {
		return false, nil
	}
	db.aiSettings.LastScheduledRun = &slot
	return true, nil
}

func (db *MockDB) ReleaseAIReportSlot(_ context.Context, slot time.Time) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	if db.aiSettings != nil && db.aiSettings.LastScheduledRun != nil && db.aiSettings.LastScheduledRun.Equal(slot) {
		db.aiSettings.LastScheduledRun = nil
	}
	return nil
}

func (db *MockDB) SaveAIReport(_ context.Context, report *entity.AIReport) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	r := *report
	r.Analyses = append([]*entity.AIAnalysis(nil), report.Analyses...)
	r.Deliveries = append([]*entity.AIReportDelivery(nil), report.Deliveries...)
	r.Scopes = append([]*entity.AIReportScope(nil), report.Scopes...)
	db.aiReports[r.Id] = &r
	return nil
}

func (db *MockDB) GetAIReport(_ context.Context, id string) (*entity.AIReport, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	r, ok := db.aiReports[id]
	if !ok {
		return nil, nil
	}
	c := *r
	return &c, nil
}

func (db *MockDB) ListAIReports(_ context.Context, limit int) ([]*entity.AIReport, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	list := make([]*entity.AIReport, 0, len(db.aiReports))
	for _, r := range db.aiReports {
		c := *r
		c.Snapshot = ""
		c.Scopes = nil
		for _, sc := range r.Scopes {
			light := *sc
			light.Snapshot = ""
			c.Scopes = append(c.Scopes, &light)
		}
		list = append(list, &c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

func (db *MockDB) SaveAIUsage(_ context.Context, usage *entity.AIUsage) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	u := *usage
	db.aiUsage = append(db.aiUsage, &u)
	return nil
}

func (db *MockDB) ListAIUsage(_ context.Context, from, to time.Time) ([]*entity.AIUsage, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	var list []*entity.AIUsage
	for _, u := range db.aiUsage {
		if !u.Time.Before(from) && !u.Time.After(to) {
			c := *u
			list = append(list, &c)
		}
	}
	return list, nil
}

func (db *MockDB) ListMailSubscriptionsByReport(_ context.Context, report string) ([]*entity.MailSubscription, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	var list []*entity.MailSubscription
	for _, s := range db.mailSubscriptions {
		r := s.Report
		if r == "" {
			r = entity.MailReportStatistic
		}
		if r == report {
			c := *s
			list = append(list, &c)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.Before(list[j].CreatedAt) })
	return list, nil
}

func (db *MockDB) GetDefaultPaymentPlan(_ context.Context) (*entity.PaymentPlan, error) {
	return &entity.PaymentPlan{PlanId: "default", IsDefault: true, IsActive: true, PricePerKwh: 45}, nil
}
