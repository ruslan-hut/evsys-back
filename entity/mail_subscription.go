package entity

import (
	"evsys-back/internal/lib/validate"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	MailPeriodDaily   = "daily"
	MailPeriodWeekly  = "weekly"
	MailPeriodMonthly = "monthly"
)

// Report types a subscription can carry. Documents written before the field
// existed have none and are statistic subscriptions.
const (
	MailReportStatistic  = "statistic"
	MailReportAIAnalysis = "ai_analysis"
)

// Languages a report can be written in; Spanish is the default.
const (
	MailLanguageES = "es"
	MailLanguageEN = "en"
)

// MailLanguages lists the supported report languages, the default first.
func MailLanguages() []string {
	return []string{MailLanguageES, MailLanguageEN}
}

type MailSubscription struct {
	Id     string `json:"id" bson:"_id,omitempty"`
	Email  string `json:"email" bson:"email" validate:"required,email_rfc"`
	Report string `json:"report" bson:"report" validate:"omitempty,oneof=statistic ai_analysis"`
	// Period and UserGroup apply to statistic reports and are required there
	// (checked once the stored values are merged, see core). The AI analysis
	// covers the whole network on the schedule of its own settings and has
	// neither, which also keeps it out of the statistic scheduler.
	Period    string `json:"period" bson:"period" validate:"omitempty,oneof=daily weekly monthly"`
	UserGroup string `json:"user_group" bson:"user_group"`
	Language  string `json:"language" bson:"language" validate:"omitempty,oneof=es en"`
	// LocationIds limits an AI analysis to these locations; empty covers the
	// whole network. On update, an absent field keeps the stored list and an
	// empty one clears it.
	LocationIds []string  `json:"location_ids,omitempty" bson:"location_ids,omitempty" validate:"omitempty,max=50,dive,required,max=39"`
	Enabled     bool      `json:"enabled" bson:"enabled"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
}

func (m *MailSubscription) Bind(_ *http.Request) error {
	return validate.Struct(m)
}

// IsAIAnalysis reports whether the subscription receives the AI analysis.
func (m *MailSubscription) IsAIAnalysis() bool {
	return m.Report == MailReportAIAnalysis
}

// Normalize fills the defaults of a subscription - the statistic report and
// the Spanish language - and clears the period and group of an AI analysis.
// Applied on write and on read, so that documents older than the fields look
// like new ones.
func (m *MailSubscription) Normalize() {
	if m.Report == "" {
		m.Report = MailReportStatistic
	}
	if m.Language == "" {
		m.Language = MailLanguageES
	}
	if m.IsAIAnalysis() {
		m.Period = ""
		m.UserGroup = ""
		m.LocationIds = NormalizeLocationIds(m.LocationIds)
	} else {
		m.LocationIds = nil
	}
}

// NormalizeLocationIds sorts and deduplicates location ids; an empty list
// becomes nil, the whole network.
func NormalizeLocationIds(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// ScopeKey names the set of locations a report covers: "" for the whole
// network, the sorted ids joined by commas otherwise.
func ScopeKey(locationIds []string) string {
	return strings.Join(NormalizeLocationIds(locationIds), ",")
}

// ReportLocation is a location as the report settings offer it.
type ReportLocation struct {
	Id           string   `json:"id"`
	Name         string   `json:"name"`
	City         string   `json:"city,omitempty"`
	ChargePoints []string `json:"charge_points"`
}
