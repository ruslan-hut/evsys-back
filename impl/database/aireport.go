package database

import (
	"context"
	"errors"
	"evsys-back/entity"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// owned by this service: the AI analysis, see impl/aireport
const (
	collectionAISettings = "ai_settings"
	collectionAIReports  = "ai_reports"
	collectionAIUsage    = "ai_usage"

	aiReportSettingsId = "ai_report"
)

// GetAIReportSettings returns the stored settings, or nil when none are.
func (m *MongoDB) GetAIReportSettings(ctx context.Context) (*entity.AIReportSettings, error) {
	return findOne[entity.AIReportSettings](m, ctx, collectionAISettings, bson.D{{Key: "_id", Value: aiReportSettingsId}})
}

// SaveAIReportSettings stores the settings, keeping the last scheduled run:
// that field belongs to the scheduler's claim, not to the admin's edit.
func (m *MongoDB) SaveAIReportSettings(ctx context.Context, settings *entity.AIReportSettings) error {
	update := bson.M{"$set": bson.M{
		"enabled":            settings.Enabled,
		"model":              settings.Model,
		"reasoning_effort":   settings.ReasoningEffort,
		"max_output_tokens":  settings.MaxOutputTokens,
		"system_prompt":      settings.SystemPrompt,
		"weekday":            settings.Weekday,
		"hour":               settings.Hour,
		"time_zone":          settings.TimeZone,
		"period_days":        settings.PeriodDays,
		"monthly_budget_usd": settings.MonthlyBudgetUSD,
		"updated_at":         settings.UpdatedAt,
		"updated_by":         settings.UpdatedBy,
	}}
	_, err := m.col(collectionAISettings).UpdateOne(ctx, bson.D{{Key: "_id", Value: aiReportSettingsId}}, update,
		options.Update().SetUpsert(true))
	return err
}

// ClaimAIReportSlot sets the last scheduled run to slot if it is earlier or
// unset, atomically, so that two instances never both run one slot.
func (m *MongoDB) ClaimAIReportSlot(ctx context.Context, slot time.Time) (bool, error) {
	filter := bson.D{
		{Key: "_id", Value: aiReportSettingsId},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "last_scheduled_run", Value: bson.D{{Key: "$exists", Value: false}}}},
			bson.D{{Key: "last_scheduled_run", Value: bson.D{{Key: "$lt", Value: slot}}}},
		}},
	}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "last_scheduled_run", Value: slot}}}}
	result, err := m.col(collectionAISettings).UpdateOne(ctx, filter, update)
	if err != nil {
		return false, err
	}
	return result.ModifiedCount == 1, nil
}

// ReleaseAIReportSlot clears the last scheduled run if it is still slot.
func (m *MongoDB) ReleaseAIReportSlot(ctx context.Context, slot time.Time) error {
	filter := bson.D{{Key: "_id", Value: aiReportSettingsId}, {Key: "last_scheduled_run", Value: slot}}
	update := bson.D{{Key: "$unset", Value: bson.D{{Key: "last_scheduled_run", Value: ""}}}}
	_, err := m.col(collectionAISettings).UpdateOne(ctx, filter, update)
	return err
}

// SaveAIReport inserts or replaces a report.
func (m *MongoDB) SaveAIReport(ctx context.Context, report *entity.AIReport) error {
	_, err := m.col(collectionAIReports).ReplaceOne(ctx, bson.D{{Key: "_id", Value: report.Id}}, report,
		options.Replace().SetUpsert(true))
	return err
}

// GetAIReport returns one report with its snapshot, or nil.
func (m *MongoDB) GetAIReport(ctx context.Context, id string) (*entity.AIReport, error) {
	return findOne[entity.AIReport](m, ctx, collectionAIReports, bson.D{{Key: "_id", Value: id}})
}

// ListAIReports returns the latest reports, newest first, without snapshots.
func (m *MongoDB) ListAIReports(ctx context.Context, limit int) ([]*entity.AIReport, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetProjection(bson.D{{Key: "snapshot", Value: 0}, {Key: "scopes.snapshot", Value: 0}})
	return findMany[*entity.AIReport](m, ctx, collectionAIReports, bson.D{}, opts)
}

// SaveAIUsage stores one request record.
func (m *MongoDB) SaveAIUsage(ctx context.Context, usage *entity.AIUsage) error {
	_, err := m.col(collectionAIUsage).InsertOne(ctx, usage)
	return err
}

// ListAIUsage returns the request records of [from, to].
func (m *MongoDB) ListAIUsage(ctx context.Context, from, to time.Time) ([]*entity.AIUsage, error) {
	filter := bson.D{{Key: "time", Value: bson.D{{Key: "$gte", Value: from}, {Key: "$lte", Value: to}}}}
	opts := options.Find().SetSort(bson.D{{Key: "time", Value: -1}})
	return findMany[*entity.AIUsage](m, ctx, collectionAIUsage, filter, opts)
}

// EnsureAIReportIndexes creates the indexes of the report and usage
// collections; safe to call on every start.
func (m *MongoDB) EnsureAIReportIndexes(ctx context.Context) error {
	if _, err := m.col(collectionAIReports).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "created_at", Value: -1}},
	}); err != nil {
		return err
	}
	_, err := m.col(collectionAIUsage).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "time", Value: -1}},
	})
	return err
}

// GetDefaultPaymentPlan returns the active plan every user gets unless they
// have their own, or nil.
func (m *MongoDB) GetDefaultPaymentPlan(ctx context.Context) (*entity.PaymentPlan, error) {
	filter := bson.D{{Key: "is_default", Value: true}, {Key: "is_active", Value: true}}
	return findOne[entity.PaymentPlan](m, ctx, collectionPaymentPlans, filter)
}

// ListMailSubscriptionsByReport returns the subscriptions of one report type.
// Documents written before the report field existed are statistic ones.
func (m *MongoDB) ListMailSubscriptionsByReport(ctx context.Context, report string) ([]*entity.MailSubscription, error) {
	if report == "" {
		return nil, errors.New("report type required")
	}
	filter := bson.D{{Key: "report", Value: report}}
	if report == entity.MailReportStatistic {
		filter = bson.D{{Key: "report", Value: bson.D{{Key: "$in", Value: bson.A{report, "", nil}}}}}
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	return findMany[*entity.MailSubscription](m, ctx, collectionMailSubscriptions, filter, opts)
}
