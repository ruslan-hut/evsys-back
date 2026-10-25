// Package aireport runs the weekly AI analysis of the network: it collects a
// snapshot of the period from the database, asks OpenAI for a structured
// report in each language its subscribers read, emails it and records every
// request with its cost.
package aireport

import (
	"context"
	"encoding/json"
	"errors"
	"evsys-back/entity"
	"evsys-back/impl/openai"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	tickInterval = time.Minute
	// runTimeout bounds one run: data collection, a model request per
	// language and the emails.
	runTimeout = 30 * time.Minute
	reportList = 30
)

// ErrBusy means another run is in progress.
var ErrBusy = errors.New("an analysis is already running")

// ErrStopped means the service is shutting down.
var ErrStopped = errors.New("the ai report service is stopping")

// Repository is what the service reads and writes.
type Repository interface {
	GetAIReportSettings(ctx context.Context) (*entity.AIReportSettings, error)
	SaveAIReportSettings(ctx context.Context, settings *entity.AIReportSettings) error
	// ClaimAIReportSlot records slot as the last scheduled run unless it, or
	// a later one, is recorded already; it reports whether this call did.
	ClaimAIReportSlot(ctx context.Context, slot time.Time) (bool, error)
	// ReleaseAIReportSlot undoes the claim of slot, if it is still the last
	// one, so that the scheduler runs it again.
	ReleaseAIReportSlot(ctx context.Context, slot time.Time) error
	SaveAIReport(ctx context.Context, report *entity.AIReport) error
	GetAIReport(ctx context.Context, id string) (*entity.AIReport, error)
	// ListAIReports returns the latest reports without their snapshots.
	ListAIReports(ctx context.Context, limit int) ([]*entity.AIReport, error)
	SaveAIUsage(ctx context.Context, usage *entity.AIUsage) error
	ListAIUsage(ctx context.Context, from, to time.Time) ([]*entity.AIUsage, error)
	ListMailSubscriptionsByReport(ctx context.Context, report string) ([]*entity.MailSubscription, error)

	GetChargePoints(ctx context.Context, level int, searchTerm string) ([]*entity.ChargePoint, error)
	StationUptime(ctx context.Context, from, to time.Time, chargePointId string) ([]*entity.StationUptime, error)
	ErrorSummary(ctx context.Context, from, to time.Time, chargePointId string) ([]*entity.ErrorSummary, error)
	ReadLog(ctx context.Context, name string, filter *entity.LogFilter) (any, error)
	GetFilteredTransactions(ctx context.Context, filter *entity.TransactionFilter) ([]*entity.Transaction, error)
	GetAllPaymentRetries(ctx context.Context) ([]*entity.PaymentRetry, error)
	GetWebhookOutboxStats(ctx context.Context) ([]*entity.WebhookOutboxStats, error)
	ListWebhookProblemDeliveries(ctx context.Context, limit int) ([]*entity.WebhookDeliveryView, error)
}

// Model answers one request; implemented by openai.Client.
type Model interface {
	Configured() bool
	Respond(ctx context.Context, req openai.Request) (*openai.Response, error)
}

// Sender delivers one email; implemented by brevo.Client.
type Sender interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}

// Service runs the analysis on schedule and on demand.
type Service struct {
	repo   Repository
	model  Model
	sender Sender
	log    *slog.Logger
	now    func() time.Time
	newId  func() string

	// running admits one run at a time, scheduled or manual
	running sync.Mutex
	// stop is closed by Stop; the scheduler and every run watch it
	stop     chan struct{}
	stopOnce sync.Once
	started  sync.Once
	// wg counts the scheduler and the runs in progress, which Stop waits for
	wg sync.WaitGroup
}

// New builds the service. sender may be nil when mail is not configured: the
// analysis can then be run and previewed, but not sent.
func New(repo Repository, model Model, sender Sender, log *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		model:  model,
		sender: sender,
		log:    log.With(sl.Module("impl.aireport")),
		now:    time.Now,
		newId:  func() string { return primitive.NewObjectID().Hex() },
		stop:   make(chan struct{}),
	}
}

// Start closes the runs a previous process left unfinished and launches the
// scheduler goroutine, which checks once a minute whether a scheduled run is
// due.
func (s *Service) Start() {
	s.started.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		s.recoverInterrupted(ctx)
		cancel()

		s.wg.Add(1)
		go s.loop()
		s.log.Info("ai report scheduler started")
	})
}

// Stop ends the scheduler, cancels the runs in progress and waits until they
// have recorded how far they got.
func (s *Service) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
	s.wg.Wait()
}

// runContext returns the context of one run: bounded by runTimeout and
// cancelled when the service stops.
func (s *Service) runContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	go func() {
		select {
		case <-s.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func (s *Service) stopping() bool {
	select {
	case <-s.stop:
		return true
	default:
		return false
	}
}

func (s *Service) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		s.tick()
		select {
		case <-s.stop:
			s.log.Info("ai report scheduler stopped")
			return
		case <-ticker.C:
		}
	}
}

// recoverInterrupted fails the runs left "running" by a process that stopped
// mid-run. A scheduled run that had not sent anything gives its slot back, so
// the scheduler runs it again while the catch-up window is open.
func (s *Service) recoverInterrupted(ctx context.Context) {
	reports, err := s.repo.ListAIReports(ctx, reportList)
	if err != nil {
		s.log.Error("list ai reports", sl.Err(err))
		return
	}
	for _, r := range reports {
		if r.Status != entity.AIReportRunning {
			continue
		}
		full, err := s.repo.GetAIReport(ctx, r.Id)
		if err != nil || full == nil {
			s.log.Error("read interrupted ai report", slog.String("report", r.Id), sl.Err(err))
			continue
		}
		s.finish(full, errors.New("interrupted: the service stopped during the run"))
		if full.Slot != nil && len(full.Deliveries) == 0 {
			if err = s.repo.ReleaseAIReportSlot(ctx, *full.Slot); err != nil {
				s.log.Error("release ai report slot", sl.Err(err))
			}
		}
		s.log.Warn("interrupted ai report closed", slog.String("report", full.Id))
	}
}

// tick starts the scheduled run when its slot has come and nobody has claimed
// it, catching up on a slot missed by at most catchUpWindow.
func (s *Service) tick() {
	ctx, cancel := s.runContext()
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("panic in ai report scheduler", slog.Any("panic", r), slog.String("stack", string(debug.Stack())))
		}
	}()
	settings, stored, err := s.loadSettings(ctx)
	if err != nil {
		s.log.Error("load ai report settings", sl.Err(err))
		return
	}
	if !settings.Enabled || !s.model.Configured() || s.sender == nil {
		return
	}
	loc, err := location(settings)
	if err != nil {
		s.log.Error("ai report schedule", sl.Err(err))
		return
	}
	now := s.now()
	slot := lastSlot(now, time.Weekday(settings.Weekday), settings.Hour, loc)
	if now.Sub(slot) > catchUpWindow {
		return
	}
	if settings.LastScheduledRun != nil && !settings.LastScheduledRun.Before(slot) {
		return
	}
	// a slot that passed before the settings were last saved is not caught
	// up: enabling the analysis or moving its hour must not send at once
	if slot.Before(settings.UpdatedAt) {
		return
	}
	// a manual run in progress delays the slot to a later tick
	if !s.running.TryLock() {
		return
	}
	defer s.running.Unlock()
	if !stored {
		// the claim updates the settings document, which must exist
		if err = s.repo.SaveAIReportSettings(ctx, settings); err != nil {
			s.log.Error("store default ai report settings", sl.Err(err))
			return
		}
	}
	claimed, err := s.repo.ClaimAIReportSlot(ctx, slot.UTC())
	if err != nil {
		s.log.Error("claim ai report slot", sl.Err(err))
		return
	}
	if !claimed {
		return
	}

	recipients, err := s.recipients(ctx)
	if err != nil {
		s.log.Error("list ai report recipients", sl.Err(err))
		return
	}
	if len(recipients) == 0 {
		s.log.Info("scheduled ai report skipped: no subscribers", slog.Time("slot", slot))
		return
	}

	report := s.newReport(settings, loc, slot, entity.AITriggerSchedule, "", false)
	slotUTC := slot.UTC()
	report.Slot = &slotUTC
	if err = s.repo.SaveAIReport(ctx, report); err != nil {
		s.log.Error("store ai report", sl.Err(err))
		if rerr := s.repo.ReleaseAIReportSlot(ctx, slotUTC); rerr != nil {
			s.log.Error("release ai report slot", sl.Err(rerr))
		}
		return
	}
	s.execute(ctx, settings, report, languagesOf(recipients), recipients)
}

// recipient is one address and the language it reads.
type recipient struct {
	email, language string
}

func (s *Service) recipients(ctx context.Context) ([]recipient, error) {
	subs, err := s.repo.ListMailSubscriptionsByReport(ctx, entity.MailReportAIAnalysis)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []recipient
	for _, sub := range subs {
		key := strings.ToLower(strings.TrimSpace(sub.Email))
		if !sub.Enabled || seen[key] {
			continue
		}
		seen[key] = true
		lang := sub.Language
		if _, ok := languageNames[lang]; !ok {
			lang = entity.MailLanguageES
		}
		out = append(out, recipient{email: sub.Email, language: lang})
	}
	return out, nil
}

func languagesOf(recipients []recipient) []string {
	seen := make(map[string]bool)
	var out []string
	for _, lang := range entity.MailLanguages() {
		for _, r := range recipients {
			if r.language == lang && !seen[lang] {
				seen[lang] = true
				out = append(out, lang)
			}
		}
	}
	return out
}

// loadSettings returns the stored settings or the defaults, and whether they
// are stored.
func (s *Service) loadSettings(ctx context.Context) (*entity.AIReportSettings, bool, error) {
	settings, err := s.repo.GetAIReportSettings(ctx)
	if err != nil {
		return nil, false, err
	}
	if settings == nil {
		return entity.DefaultAIReportSettings(), false, nil
	}
	return settings, true, nil
}

func (s *Service) newReport(settings *entity.AIReportSettings, loc *time.Location, at time.Time, trigger, author string, dryRun bool) *entity.AIReport {
	from, to := periodRange(at, settings.PeriodDays, loc)
	return &entity.AIReport{
		Id:         s.newId(),
		Trigger:    trigger,
		Author:     author,
		DryRun:     dryRun,
		Status:     entity.AIReportRunning,
		PeriodFrom: from.UTC(),
		PeriodTo:   to.UTC(),
		TimeZone:   settings.TimeZone,
		Model:      settings.Model,
		CreatedAt:  s.now().UTC(),
	}
}

// execute fills a stored report: collects the snapshot, asks the model in each
// language and sends the emails. The report is saved after every step, so the
// admin page shows the progress and a failure leaves what was done.
func (s *Service) execute(ctx context.Context, settings *entity.AIReportSettings, report *entity.AIReport, languages []string, recipients []recipient) {
	log := s.log.With(slog.String("report", report.Id), slog.String("trigger", report.Trigger))
	defer func() {
		if r := recover(); r != nil {
			log.Error("panic in ai report run", slog.Any("panic", r), slog.String("stack", string(debug.Stack())))
			s.finish(report, fmt.Errorf("internal error: %v", r))
		}
	}()

	loc, err := location(settings)
	if err != nil {
		s.finish(report, err)
		return
	}
	prevFrom := report.PeriodFrom.In(loc).AddDate(0, 0, -settings.PeriodDays)
	snap := s.collect(ctx, report.PeriodFrom, report.PeriodTo, prevFrom, settings.TimeZone)
	data, err := encodeSnapshot(snap)
	if err != nil {
		s.finish(report, fmt.Errorf("encode snapshot: %w", err))
		return
	}
	report.Snapshot = string(data)
	report.SnapshotBytes = len(data)
	s.save(ctx, report)
	log.Info("ai report data collected", slog.Int("bytes", len(data)), slog.Int("notes", len(snap.Notes)))

	if report.DryRun {
		s.finish(report, nil)
		return
	}
	if !s.model.Configured() {
		s.finish(report, openai.ErrNotConfigured)
		return
	}

	for _, lang := range languages {
		if err = s.checkBudget(ctx, settings); err != nil {
			report.Analyses = append(report.Analyses, &entity.AIAnalysis{Language: lang, Error: err.Error()})
			break
		}
		analysis := s.analyse(ctx, settings, report, data, lang)
		report.Analyses = append(report.Analyses, analysis)
		s.save(ctx, report)
	}

	for _, r := range recipients {
		report.Deliveries = append(report.Deliveries, s.deliver(ctx, report, snap, r))
	}

	var failed error
	if !hasResult(report) {
		failed = errors.New("no analysis succeeded")
		for _, a := range report.Analyses {
			if a.Error != "" {
				failed = errors.New(a.Error)
				break
			}
		}
	}
	s.finish(report, failed)
	log.Info("ai report finished", slog.String("status", report.Status), slog.Float64("cost_usd", report.CostUSD),
		slog.Int("deliveries", len(report.Deliveries)))
}

func hasResult(report *entity.AIReport) bool {
	for _, a := range report.Analyses {
		if a.Result != nil {
			return true
		}
	}
	return false
}

func analysisIn(report *entity.AIReport, language string) *entity.AIAnalysis {
	for _, a := range report.Analyses {
		if a.Language == language && a.Result != nil {
			return a
		}
	}
	return nil
}

// analyse asks the model for the report in one language and records the
// request's usage, whether it succeeded or not.
func (s *Service) analyse(ctx context.Context, settings *entity.AIReportSettings, report *entity.AIReport, snapshot []byte, language string) *entity.AIAnalysis {
	instructions := settings.SystemPrompt
	if instructions == "" {
		instructions = DefaultSystemPrompt
	}
	analysis := &entity.AIAnalysis{Language: language}
	resp, usage, err := s.request(ctx, settings, report.Trigger, report.Id, language, openai.Request{
		Model:           settings.Model,
		Instructions:    instructions,
		Input:           analysisInput(snapshot, language),
		SchemaName:      "evsys_weekly_analysis",
		Schema:          analysisSchema,
		MaxOutputTokens: settings.MaxOutputTokens,
		ReasoningEffort: settings.ReasoningEffort,
	})
	if usage != nil {
		report.CostUSD += usage.CostUSD
	}
	if resp != nil {
		analysis.ResponseId = resp.Id
	}
	if err != nil {
		analysis.Error = err.Error()
		return analysis
	}
	var result entity.AIAnalysisResult
	if err = json.Unmarshal([]byte(resp.Text), &result); err != nil {
		analysis.Error = fmt.Sprintf("decode answer: %v", err)
		return analysis
	}
	sortFindings(result.Findings)
	analysis.Result = &result
	return analysis
}

var severityRank = map[string]int{"critical": 0, "warning": 1, "info": 2}

func sortFindings(findings []*entity.AIFinding) {
	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank[findings[i].Severity] < severityRank[findings[j].Severity]
	})
}

// request makes one model request and stores its usage record.
func (s *Service) request(ctx context.Context, settings *entity.AIReportSettings, trigger, reportId, language string, req openai.Request) (*openai.Response, *entity.AIUsage, error) {
	started := s.now()
	resp, err := s.model.Respond(ctx, req)
	usage := &entity.AIUsage{
		Id:         s.newId(),
		Time:       started.UTC(),
		Trigger:    trigger,
		ReportId:   reportId,
		Model:      req.Model,
		Language:   language,
		DurationMs: s.now().Sub(started).Milliseconds(),
		Success:    err == nil,
	}
	if err != nil {
		usage.Error = err.Error()
	}
	if resp != nil {
		usage.ResponseId = resp.Id
		usage.InputTokens = resp.Usage.InputTokens
		usage.CachedTokens = resp.Usage.CachedTokens
		usage.OutputTokens = resp.Usage.OutputTokens
		usage.ReasoningTokens = resp.Usage.ReasoningTokens
		usage.CostUSD = cost(req.Model, resp.Usage)
	}
	// the usage is what the budget counts: store it even when the run is
	// being cancelled
	storeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if serr := s.repo.SaveAIUsage(storeCtx, usage); serr != nil {
		s.log.Error("store ai usage", sl.Err(serr))
	}
	return resp, usage, err
}

// cost prices a request by the model's list prices; cached input is billed at
// its own rate and reasoning is part of the output.
func cost(model string, u openai.Usage) float64 {
	m, ok := entity.FindAIModel(model)
	if !ok {
		// a model dropped from the price list must still count against the
		// budget: price it as the dearest one
		for _, o := range entity.AIModels() {
			if o.OutputPrice > m.OutputPrice {
				m = o
			}
		}
	}
	uncached := u.InputTokens - u.CachedTokens
	if uncached < 0 {
		uncached = 0
	}
	return (float64(uncached)*m.InputPrice + float64(u.CachedTokens)*m.CachedPrice + float64(u.OutputTokens)*m.OutputPrice) / 1_000_000
}

// checkBudget refuses a request once the month's spend reaches the budget.
func (s *Service) checkBudget(ctx context.Context, settings *entity.AIReportSettings) error {
	if settings.MonthlyBudgetUSD <= 0 {
		return nil
	}
	spent, err := s.monthSpent(ctx)
	if err != nil {
		return fmt.Errorf("read monthly spend: %w", err)
	}
	if spent >= settings.MonthlyBudgetUSD {
		return fmt.Errorf("monthly budget of %.2f USD reached (spent %.2f USD)", settings.MonthlyBudgetUSD, spent)
	}
	return nil
}

func (s *Service) monthSpent(ctx context.Context) (float64, error) {
	now := s.now()
	records, err := s.repo.ListAIUsage(ctx, monthStart(now), now)
	if err != nil {
		return 0, err
	}
	total := 0.0
	for _, r := range records {
		total += r.CostUSD
	}
	return total, nil
}

// deliver sends the analysis in the recipient's language, falling back to
// any language that succeeded.
func (s *Service) deliver(ctx context.Context, report *entity.AIReport, snap *Snapshot, r recipient) *entity.AIReportDelivery {
	d := &entity.AIReportDelivery{Email: r.email, Language: r.language, SentAt: s.now().UTC()}
	if s.sender == nil {
		d.Error = "mail service not configured"
		return d
	}
	analysis := analysisIn(report, r.language)
	if analysis == nil {
		for _, a := range report.Analyses {
			if a.Result != nil {
				analysis = a
				break
			}
		}
	}
	if analysis == nil {
		d.Error = "no analysis to send"
		return d
	}
	d.Language = analysis.Language
	subject := subjectOf(report, analysis.Result, analysis.Language)
	if err := s.sender.Send(ctx, r.email, subject, renderReport(report, analysis, snap)); err != nil {
		d.Error = err.Error()
		s.log.Error("send ai report", slog.String("report", report.Id), slog.String("to", r.email), sl.Err(err))
	}
	return d
}

func (s *Service) save(ctx context.Context, report *entity.AIReport) {
	if err := s.repo.SaveAIReport(ctx, report); err != nil {
		s.log.Error("store ai report", slog.String("report", report.Id), sl.Err(err))
	}
}

// finish closes a report as done or failed. It saves with a context of its
// own, so that a cancelled run is still recorded.
func (s *Service) finish(report *entity.AIReport, err error) {
	now := s.now().UTC()
	report.FinishedAt = &now
	report.Status = entity.AIReportDone
	if err != nil {
		report.Status = entity.AIReportFailed
		report.Error = err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.save(ctx, report)
}

// Run starts an analysis by hand and returns its report at once; the run goes
// on in the background and the report is updated as it progresses.
func (s *Service) Run(ctx context.Context, author string, req *entity.AIReportRunRequest) (*entity.AIReport, error) {
	settings, _, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	loc, err := location(settings)
	if err != nil {
		return nil, err
	}
	if !req.DryRun && !s.model.Configured() {
		return nil, openai.ErrNotConfigured
	}

	var languages []string
	var recipients []recipient
	switch {
	case req.DryRun:
	case req.Email != "":
		lang := req.Language
		if lang == "" {
			lang = entity.MailLanguageES
		}
		languages = []string{lang}
		recipients = []recipient{{email: req.Email, language: lang}}
	case req.SendAll:
		if recipients, err = s.recipients(ctx); err != nil {
			return nil, err
		}
		if len(recipients) == 0 {
			return nil, errors.New("no enabled subscribers of the ai analysis")
		}
		languages = languagesOf(recipients)
	default:
		lang := req.Language
		if lang == "" {
			lang = entity.MailLanguageES
		}
		languages = []string{lang}
	}
	if len(recipients) > 0 && s.sender == nil {
		return nil, errors.New("mail service not configured")
	}
	if !req.DryRun {
		if err = s.checkBudget(ctx, settings); err != nil {
			return nil, err
		}
	}

	if s.stopping() {
		return nil, ErrStopped
	}
	if !s.running.TryLock() {
		return nil, ErrBusy
	}
	report := s.newReport(settings, loc, s.now(), entity.AITriggerManual, author, req.DryRun)
	if err = s.repo.SaveAIReport(ctx, report); err != nil {
		s.running.Unlock()
		return nil, err
	}
	// the caller gets the report as it starts; the run changes its own copy
	started := *report
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.running.Unlock()
		runCtx, cancel := s.runContext()
		defer cancel()
		s.execute(runCtx, settings, report, languages, recipients)
	}()
	return &started, nil
}

// SendTo emails the analysis to one subscriber, in their language. A report
// of the current period that already has that language is sent again;
// otherwise a new one is generated. Runs in the background.
func (s *Service) SendTo(ctx context.Context, author string, sub *entity.MailSubscription) error {
	if s.sender == nil {
		return errors.New("mail service not configured")
	}
	lang := sub.Language
	if _, ok := languageNames[lang]; !ok {
		lang = entity.MailLanguageES
	}
	settings, _, err := s.loadSettings(ctx)
	if err != nil {
		return err
	}
	loc, err := location(settings)
	if err != nil {
		return err
	}
	_, to := periodRange(s.now(), settings.PeriodDays, loc)

	reports, err := s.repo.ListAIReports(ctx, reportList)
	if err != nil {
		return err
	}
	for _, r := range reports {
		if r.Status != entity.AIReportDone || r.DryRun || !r.PeriodTo.Equal(to.UTC()) || analysisIn(r, lang) == nil {
			continue
		}
		full, err := s.repo.GetAIReport(ctx, r.Id)
		if err != nil {
			return err
		}
		if full == nil {
			continue
		}
		s.wg.Add(1)
		go s.resend(full, recipient{email: sub.Email, language: lang})
		return nil
	}

	_, err = s.Run(ctx, author, &entity.AIReportRunRequest{Email: sub.Email, Language: lang})
	return err
}

func (s *Service) resend(report *entity.AIReport, r recipient) {
	defer s.wg.Done()
	ctx, cancel := s.runContext()
	defer cancel()
	d := s.deliver(ctx, report, decodeSnapshot(report), r)
	report.Deliveries = append(report.Deliveries, d)
	s.save(ctx, report)
}

func decodeSnapshot(report *entity.AIReport) *Snapshot {
	if report.Snapshot == "" {
		return nil
	}
	var snap Snapshot
	if err := json.Unmarshal([]byte(report.Snapshot), &snap); err != nil {
		return nil
	}
	return &snap
}

// Preview renders the email of a report in one language.
func (s *Service) Preview(ctx context.Context, id, language string) (*entity.AIReportPreview, error) {
	report, err := s.repo.GetAIReport(ctx, id)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, fmt.Errorf("report %w", entity.ErrNotFound)
	}
	analysis := analysisIn(report, language)
	if analysis == nil {
		for _, a := range report.Analyses {
			if a.Result != nil {
				analysis = a
				break
			}
		}
	}
	if analysis == nil {
		return nil, fmt.Errorf("report has no analysis: %w", entity.ErrNotFound)
	}
	return &entity.AIReportPreview{
		Language: analysis.Language,
		Subject:  subjectOf(report, analysis.Result, analysis.Language),
		Html:     renderReport(report, analysis, decodeSnapshot(report)),
	}, nil
}

// Report returns one report with its snapshot.
func (s *Service) Report(ctx context.Context, id string) (*entity.AIReport, error) {
	report, err := s.repo.GetAIReport(ctx, id)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, fmt.Errorf("report %w", entity.ErrNotFound)
	}
	return report, nil
}

// Reports lists the latest reports.
func (s *Service) Reports(ctx context.Context, limit int) ([]*entity.AIReport, error) {
	if limit <= 0 || limit > 100 {
		limit = reportList
	}
	return s.repo.ListAIReports(ctx, limit)
}

// Settings returns the settings with what the admin page needs around them.
func (s *Service) Settings(ctx context.Context) (*entity.AIReportSettingsView, error) {
	settings, _, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	view := &entity.AIReportSettingsView{
		AIReportSettings:    settings,
		HasApiKey:           s.model.Configured(),
		MailEnabled:         s.sender != nil,
		DefaultSystemPrompt: DefaultSystemPrompt,
		Models:              entity.AIModels(),
		Languages:           entity.MailLanguages(),
	}
	if loc, err := location(settings); err == nil && settings.Enabled {
		next := nextSlot(s.now(), time.Weekday(settings.Weekday), settings.Hour, loc).UTC()
		view.NextRun = &next
	}
	if view.MonthSpentUSD, err = s.monthSpent(ctx); err != nil {
		return nil, err
	}
	recipients, err := s.recipients(ctx)
	if err != nil {
		return nil, err
	}
	view.Recipients = len(recipients)
	return view, nil
}

// UpdateSettings applies a partial update.
func (s *Service) UpdateSettings(ctx context.Context, author string, u *entity.AIReportSettingsUpdate) (*entity.AIReportSettingsView, error) {
	settings, _, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	if u.Enabled != nil {
		settings.Enabled = *u.Enabled
	}
	if u.Model != nil {
		if _, ok := entity.FindAIModel(*u.Model); !ok {
			return nil, fmt.Errorf("unknown model %q", *u.Model)
		}
		settings.Model = *u.Model
	}
	if u.ReasoningEffort != nil {
		settings.ReasoningEffort = *u.ReasoningEffort
	}
	if u.MaxOutputTokens != nil {
		settings.MaxOutputTokens = *u.MaxOutputTokens
	}
	if u.SystemPrompt != nil {
		settings.SystemPrompt = *u.SystemPrompt
	}
	if u.Weekday != nil {
		settings.Weekday = *u.Weekday
	}
	if u.Hour != nil {
		settings.Hour = *u.Hour
	}
	if u.TimeZone != nil {
		settings.TimeZone = *u.TimeZone
	}
	if u.PeriodDays != nil {
		settings.PeriodDays = *u.PeriodDays
	}
	if u.MonthlyBudgetUSD != nil {
		settings.MonthlyBudgetUSD = *u.MonthlyBudgetUSD
	}
	if _, err = location(settings); err != nil {
		return nil, err
	}
	settings.UpdatedAt = s.now().UTC()
	settings.UpdatedBy = author
	if err = s.repo.SaveAIReportSettings(ctx, settings); err != nil {
		return nil, err
	}
	return s.Settings(ctx)
}

// Test sends the model a tiny request to check the key and the model; its
// usage is recorded like any other.
func (s *Service) Test(ctx context.Context) (*entity.AIUsage, error) {
	if !s.model.Configured() {
		return nil, openai.ErrNotConfigured
	}
	settings, _, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.checkBudget(ctx, settings); err != nil {
		return nil, err
	}
	_, usage, err := s.request(ctx, settings, entity.AITriggerTest, "", "", openai.Request{
		Model:           settings.Model,
		Input:           `Answer {"ok": true}.`,
		SchemaName:      "connection_test",
		Schema:          testSchema,
		MaxOutputTokens: 1000,
		ReasoningEffort: settings.ReasoningEffort,
	})
	return usage, err
}

// Usage sums the requests of a period, per month and per model.
func (s *Service) Usage(ctx context.Context, from, to time.Time) (*entity.AIUsageReport, error) {
	records, err := s.repo.ListAIUsage(ctx, from, to)
	if err != nil {
		return nil, err
	}
	report := &entity.AIUsageReport{
		From:    from,
		To:      to,
		Models:  make(map[string]*entity.AIUsageTotals),
		Records: records,
	}
	months := make(map[string]*entity.AIUsageMonth)
	for _, r := range records {
		report.Totals.Add(r)
		key := r.Time.UTC().Format("2006-01")
		m, ok := months[key]
		if !ok {
			m = &entity.AIUsageMonth{Month: key}
			months[key] = m
		}
		m.Add(r)
		t, ok := report.Models[r.Model]
		if !ok {
			t = &entity.AIUsageTotals{}
			report.Models[r.Model] = t
		}
		t.Add(r)
	}
	for _, m := range months {
		report.Months = append(report.Months, m)
	}
	sort.Slice(report.Months, func(i, j int) bool { return report.Months[i].Month > report.Months[j].Month })
	sort.Slice(report.Records, func(i, j int) bool { return report.Records[i].Time.After(report.Records[j].Time) })
	return report, nil
}
