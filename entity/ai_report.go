package entity

import (
	"evsys-back/internal/lib/validate"
	"net/http"
	"time"
)

// Defaults of the AI analysis settings, used until an admin saves their own.
const (
	AIReportDefaultModel           = "gpt-6.1-sol"
	AIReportDefaultWeekday         = int(time.Monday)
	AIReportDefaultHour            = 7
	AIReportDefaultTimeZone        = "Europe/Madrid"
	AIReportDefaultPeriodDays      = 7
	AIReportDefaultMaxOutputTokens = 8000
)

// Triggers of an AI report run, recorded on the report and on its usage.
const (
	AITriggerSchedule = "schedule"
	AITriggerManual   = "manual"
	AITriggerTest     = "test"
)

// Statuses of an AI report run.
const (
	AIReportRunning = "running"
	AIReportDone    = "done"
	AIReportFailed  = "failed"
)

// AIModelOption is one entry of the admin model picker, with the OpenAI list
// prices the spend is computed from, in USD per million tokens.
type AIModelOption struct {
	Id          string  `json:"id"`
	Label       string  `json:"label"`
	InputPrice  float64 `json:"input_price"`
	CachedPrice float64 `json:"cached_price"`
	OutputPrice float64 `json:"output_price"`
}

// AIModels returns the selectable models. Prices are OpenAI's standard tier
// for prompts under 272K tokens (developers.openai.com/api/docs/pricing,
// read 2026-10-25); update them when OpenAI reprices. Usage records keep the
// cost computed at the time of the call, so a change here does not rewrite
// history.
func AIModels() []AIModelOption {
	return []AIModelOption{
		{Id: "gpt-6.1-sol", Label: "GPT-6.1 Sol", InputPrice: 2.00, CachedPrice: 0.10, OutputPrice: 10.00},
		{Id: "gpt-6-sol", Label: "GPT-6 Sol", InputPrice: 2.00, CachedPrice: 0.20, OutputPrice: 10.00},
		{Id: "gpt-6-luna", Label: "GPT-6 Luna", InputPrice: 0.10, CachedPrice: 0.01, OutputPrice: 0.50},
		{Id: "gpt-6-astra", Label: "GPT-6 Astra", InputPrice: 10.00, CachedPrice: 1.00, OutputPrice: 50.00},
		{Id: "gpt-5.6-terra", Label: "GPT-5.6 Terra", InputPrice: 2.00, CachedPrice: 0.20, OutputPrice: 12.00},
		{Id: "gpt-5.6-luna", Label: "GPT-5.6 Luna", InputPrice: 0.20, CachedPrice: 0.02, OutputPrice: 1.20},
		{Id: "gpt-5.4-mini", Label: "GPT-5.4 mini", InputPrice: 0.75, CachedPrice: 0.075, OutputPrice: 4.50},
		{Id: "gpt-5-mini", Label: "GPT-5 mini", InputPrice: 0.25, CachedPrice: 0.025, OutputPrice: 2.00},
	}
}

// FindAIModel returns the picker entry of a model id.
func FindAIModel(id string) (AIModelOption, bool) {
	for _, m := range AIModels() {
		if m.Id == id {
			return m, true
		}
	}
	return AIModelOption{}, false
}

// AIReportSettings configures the AI analysis. The OpenAI key is not here: it
// lives in the service configuration and never reaches the database.
type AIReportSettings struct {
	Enabled bool   `json:"enabled" bson:"enabled"`
	Model   string `json:"model" bson:"model"`
	// ReasoningEffort is passed to reasoning models as is; empty leaves the
	// model's default.
	ReasoningEffort string `json:"reasoning_effort" bson:"reasoning_effort"`
	// MaxOutputTokens bounds one answer, reasoning tokens included.
	MaxOutputTokens int `json:"max_output_tokens" bson:"max_output_tokens"`
	// SystemPrompt overrides the built-in instructions; empty means the
	// default. The answer schema is enforced separately, so an edit can only
	// change the quality of the analysis, never break its parsing.
	SystemPrompt string `json:"system_prompt" bson:"system_prompt"`
	// Weekday (0 = Sunday) and Hour of the scheduled run, in TimeZone.
	Weekday  int    `json:"weekday" bson:"weekday"`
	Hour     int    `json:"hour" bson:"hour"`
	TimeZone string `json:"time_zone" bson:"time_zone"`
	// PeriodDays is how many whole days before the run the analysis covers.
	PeriodDays int `json:"period_days" bson:"period_days"`
	// MonthlyBudgetUSD stops runs once the spend of the calendar month reaches
	// it; zero means no limit.
	MonthlyBudgetUSD float64 `json:"monthly_budget_usd" bson:"monthly_budget_usd"`
	// LastScheduledRun is the scheduled slot last claimed by a run, so a
	// restart neither repeats nor skips it.
	LastScheduledRun *time.Time `json:"last_scheduled_run,omitempty" bson:"last_scheduled_run,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at" bson:"updated_at"`
	UpdatedBy        string     `json:"updated_by" bson:"updated_by"`
}

// DefaultAIReportSettings returns the settings in force before an admin saves
// any.
func DefaultAIReportSettings() *AIReportSettings {
	return &AIReportSettings{
		Enabled:         true,
		Model:           AIReportDefaultModel,
		MaxOutputTokens: AIReportDefaultMaxOutputTokens,
		Weekday:         AIReportDefaultWeekday,
		Hour:            AIReportDefaultHour,
		TimeZone:        AIReportDefaultTimeZone,
		PeriodDays:      AIReportDefaultPeriodDays,
	}
}

// AIReportSettingsUpdate is a partial update; a nil field keeps the stored
// value. SystemPrompt "" restores the built-in prompt.
type AIReportSettingsUpdate struct {
	Enabled          *bool    `json:"enabled,omitempty"`
	Model            *string  `json:"model,omitempty" validate:"omitempty,max=64"`
	ReasoningEffort  *string  `json:"reasoning_effort,omitempty" validate:"omitempty,oneof=none minimal low medium high"`
	MaxOutputTokens  *int     `json:"max_output_tokens,omitempty" validate:"omitempty,min=1000,max=64000"`
	SystemPrompt     *string  `json:"system_prompt,omitempty" validate:"omitempty,max=20000"`
	Weekday          *int     `json:"weekday,omitempty" validate:"omitempty,min=0,max=6"`
	Hour             *int     `json:"hour,omitempty" validate:"omitempty,min=0,max=23"`
	TimeZone         *string  `json:"time_zone,omitempty" validate:"omitempty,max=64"`
	PeriodDays       *int     `json:"period_days,omitempty" validate:"omitempty,min=1,max=31"`
	MonthlyBudgetUSD *float64 `json:"monthly_budget_usd,omitempty" validate:"omitempty,min=0"`
}

func (u *AIReportSettingsUpdate) Bind(_ *http.Request) error {
	return validate.Struct(u)
}

// AIReportSettingsView is what the admin page reads: the settings with what it
// needs to render them.
type AIReportSettingsView struct {
	*AIReportSettings
	// HasApiKey tells whether the service configuration carries an OpenAI key.
	HasApiKey           bool            `json:"has_api_key"`
	MailEnabled         bool            `json:"mail_enabled"`
	DefaultSystemPrompt string          `json:"default_system_prompt"`
	Models              []AIModelOption `json:"models"`
	Languages           []string        `json:"languages"`
	NextRun             *time.Time      `json:"next_run,omitempty"`
	MonthSpentUSD       float64         `json:"month_spent_usd"`
	Recipients          int             `json:"recipients"`
}

// AIReportRunRequest starts a run by hand. DryRun collects the data without
// calling the model. Without Email the report is generated in Language (or the
// languages of the subscribers when SendAll is set) and stored for preview;
// with Email it is also sent there.
type AIReportRunRequest struct {
	DryRun   bool   `json:"dry_run"`
	Language string `json:"language" validate:"omitempty,oneof=es en"`
	Email    string `json:"email" validate:"omitempty,email_rfc"`
	SendAll  bool   `json:"send_all"`
}

func (r *AIReportRunRequest) Bind(_ *http.Request) error {
	return validate.Struct(r)
}

// AIReport is one run of the analysis: the data it read, what the model
// answered per language and to whom it was sent.
type AIReport struct {
	Id         string    `json:"id" bson:"_id"`
	Trigger    string    `json:"trigger" bson:"trigger"`
	Author     string    `json:"author,omitempty" bson:"author,omitempty"`
	DryRun     bool      `json:"dry_run" bson:"dry_run"`
	Status     string    `json:"status" bson:"status"`
	Error      string    `json:"error,omitempty" bson:"error,omitempty"`
	PeriodFrom time.Time `json:"period_from" bson:"period_from"`
	PeriodTo   time.Time `json:"period_to" bson:"period_to"`
	TimeZone   string    `json:"time_zone" bson:"time_zone"`
	// Slot is the scheduled moment a scheduled run belongs to.
	Slot       *time.Time `json:"slot,omitempty" bson:"slot,omitempty"`
	Model      string     `json:"model,omitempty" bson:"model,omitempty"`
	CreatedAt  time.Time  `json:"created_at" bson:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty" bson:"finished_at,omitempty"`
	// Snapshot is the JSON the model read; left out of report lists.
	Snapshot      string              `json:"snapshot,omitempty" bson:"snapshot,omitempty"`
	SnapshotBytes int                 `json:"snapshot_bytes" bson:"snapshot_bytes"`
	Analyses      []*AIAnalysis       `json:"analyses,omitempty" bson:"analyses,omitempty"`
	Deliveries    []*AIReportDelivery `json:"deliveries,omitempty" bson:"deliveries,omitempty"`
	CostUSD       float64             `json:"cost_usd" bson:"cost_usd"`
}

// AIAnalysis is the model's answer in one language.
type AIAnalysis struct {
	Language   string            `json:"language" bson:"language"`
	Result     *AIAnalysisResult `json:"result,omitempty" bson:"result,omitempty"`
	ResponseId string            `json:"response_id,omitempty" bson:"response_id,omitempty"`
	Error      string            `json:"error,omitempty" bson:"error,omitempty"`
}

// AIAnalysisResult is the structured answer the model must give.
type AIAnalysisResult struct {
	// Status is ok, attention or critical.
	Status   string       `json:"status" bson:"status"`
	Headline string       `json:"headline" bson:"headline"`
	Summary  string       `json:"summary" bson:"summary"`
	Findings []*AIFinding `json:"findings" bson:"findings"`
	Trends   []string     `json:"trends" bson:"trends"`
}

// AIFinding is one problem or observation, ranked by severity.
type AIFinding struct {
	// Severity is critical, warning or info.
	Severity       string `json:"severity" bson:"severity"`
	Area           string `json:"area" bson:"area"`
	ChargePointId  string `json:"charge_point_id" bson:"charge_point_id"`
	Title          string `json:"title" bson:"title"`
	Details        string `json:"details" bson:"details"`
	Recommendation string `json:"recommendation" bson:"recommendation"`
}

// AIReportDelivery records one email sent from a report.
type AIReportDelivery struct {
	Email    string    `json:"email" bson:"email"`
	Language string    `json:"language" bson:"language"`
	SentAt   time.Time `json:"sent_at" bson:"sent_at"`
	Error    string    `json:"error,omitempty" bson:"error,omitempty"`
}

// AIReportPreview is a rendered report email.
type AIReportPreview struct {
	Language string `json:"language"`
	Subject  string `json:"subject"`
	Html     string `json:"html"`
}

// AIUsage is one request to the OpenAI API, kept to control the spend.
type AIUsage struct {
	Id              string    `json:"id" bson:"_id"`
	Time            time.Time `json:"time" bson:"time"`
	Trigger         string    `json:"trigger" bson:"trigger"`
	ReportId        string    `json:"report_id,omitempty" bson:"report_id,omitempty"`
	Model           string    `json:"model" bson:"model"`
	Language        string    `json:"language,omitempty" bson:"language,omitempty"`
	InputTokens     int64     `json:"input_tokens" bson:"input_tokens"`
	CachedTokens    int64     `json:"cached_tokens" bson:"cached_tokens"`
	OutputTokens    int64     `json:"output_tokens" bson:"output_tokens"`
	ReasoningTokens int64     `json:"reasoning_tokens" bson:"reasoning_tokens"`
	CostUSD         float64   `json:"cost_usd" bson:"cost_usd"`
	DurationMs      int64     `json:"duration_ms" bson:"duration_ms"`
	Success         bool      `json:"success" bson:"success"`
	Error           string    `json:"error,omitempty" bson:"error,omitempty"`
	ResponseId      string    `json:"response_id,omitempty" bson:"response_id,omitempty"`
}

// AIUsageTotals sums usage records.
type AIUsageTotals struct {
	Requests        int     `json:"requests"`
	Failed          int     `json:"failed"`
	InputTokens     int64   `json:"input_tokens"`
	CachedTokens    int64   `json:"cached_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	ReasoningTokens int64   `json:"reasoning_tokens"`
	CostUSD         float64 `json:"cost_usd"`
}

// Add counts one record in.
func (t *AIUsageTotals) Add(u *AIUsage) {
	t.Requests++
	if !u.Success {
		t.Failed++
	}
	t.InputTokens += u.InputTokens
	t.CachedTokens += u.CachedTokens
	t.OutputTokens += u.OutputTokens
	t.ReasoningTokens += u.ReasoningTokens
	t.CostUSD += u.CostUSD
}

// AIUsageMonth is the usage of one calendar month (UTC), "2026-10".
type AIUsageMonth struct {
	Month string `json:"month"`
	AIUsageTotals
}

// AIUsageReport answers the usage page: totals of the period, per month and
// per model, and the records themselves, newest first.
type AIUsageReport struct {
	From    time.Time                 `json:"from"`
	To      time.Time                 `json:"to"`
	Totals  AIUsageTotals             `json:"totals"`
	Months  []*AIUsageMonth           `json:"months"`
	Models  map[string]*AIUsageTotals `json:"models"`
	Records []*AIUsage                `json:"records"`
}
