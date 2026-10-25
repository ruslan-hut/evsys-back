package aireport

import (
	"context"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/request"
	"evsys-back/internal/lib/api/web"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// defaultUsagePeriod is the usage shown when the request names no period.
const defaultUsagePeriod = 90 * 24 * time.Hour

// Handler is the core surface the AI analysis endpoints call; every method
// admits admins only.
type Handler interface {
	GetAIReportSettings(ctx context.Context, author *entity.User) (*entity.AIReportSettingsView, error)
	UpdateAIReportSettings(ctx context.Context, author *entity.User, u *entity.AIReportSettingsUpdate) (*entity.AIReportSettingsView, error)
	TestAIReport(ctx context.Context, author *entity.User) (*entity.AIUsage, error)
	RunAIReport(ctx context.Context, author *entity.User, req *entity.AIReportRunRequest) (*entity.AIReport, error)
	ListAIReports(ctx context.Context, author *entity.User, limit int) ([]*entity.AIReport, error)
	GetAIReport(ctx context.Context, author *entity.User, id string) (*entity.AIReport, error)
	PreviewAIReport(ctx context.Context, author *entity.User, id, scope, language string) (*entity.AIReportPreview, error)
	GetAIUsage(ctx context.Context, author *entity.User, from, to time.Time) (*entity.AIUsageReport, error)
}

func loggerWith(logger *slog.Logger, r *http.Request, author *entity.User) *slog.Logger {
	return web.Log(r.Context(), logger, "handlers.aireport",
		slog.String("author", author.Username),
		slog.String("role", author.Role),
	)
}

// Settings answers GET /ai/settings.
func Settings(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		data, err := h.GetAIReportSettings(ctx, author)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to read ai report settings", err)
			return
		}
		web.OK(w, r, log, "ai report settings", data)
	}
}

// UpdateSettings answers PUT /ai/settings with a partial update.
func UpdateSettings(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		var u entity.AIReportSettingsUpdate
		if err := render.Bind(r, &u); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode settings", err)
			return
		}
		data, err := h.UpdateAIReportSettings(ctx, author, &u)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to save ai report settings", err)
			return
		}
		web.OK(w, r, log, "ai report settings updated", data)
	}
}

// Test answers POST /ai/test: a minimal OpenAI request with the stored model.
func Test(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		data, err := h.TestAIReport(ctx, author)
		if err != nil {
			web.Fail(w, r, log, 400, "OpenAI test failed", err)
			return
		}
		web.OK(w, r, log.With(slog.Float64("cost_usd", data.CostUSD)), "openai test passed", data)
	}
}

// Run answers POST /ai/reports/run with 202 and the report just started.
func Run(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		var req entity.AIReportRunRequest
		if err := render.Bind(r, &req); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode request", err)
			return
		}
		data, err := h.RunAIReport(ctx, author, &req)
		if err != nil {
			web.Fail(w, r, log, 400, "Failed to start ai report", err)
			return
		}
		render.Status(r, http.StatusAccepted)
		web.OK(w, r, log.With(slog.String("id", data.Id), slog.Bool("dry_run", req.DryRun)), "ai report started", data)
	}
}

// List answers GET /ai/reports.
func List(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		data, err := h.ListAIReports(ctx, author, limit)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to list ai reports", err)
			return
		}
		web.OK(w, r, log, "ai reports list", data)
	}
}

// Get answers GET /ai/reports/{id}.
func Get(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		id := chi.URLParam(r, "id")
		log := loggerWith(logger, r, author).With(slog.String("id", id))

		data, err := h.GetAIReport(ctx, author, id)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to read ai report", err)
			return
		}
		web.OK(w, r, log, "ai report", data)
	}
}

// Preview answers GET /ai/reports/{id}/preview?scope=&language=; scope is the
// key of one of the report's scopes, empty for the whole network.
func Preview(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		id := chi.URLParam(r, "id")
		language := r.URL.Query().Get("language")
		scope := r.URL.Query().Get("scope")
		log := loggerWith(logger, r, author).With(slog.String("id", id), slog.String("scope", scope), slog.String("language", language))

		data, err := h.PreviewAIReport(ctx, author, id, scope, language)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to render ai report", err)
			return
		}
		web.OK(w, r, log, "ai report preview", data)
	}
}

// Usage answers GET /ai/usage?from=&to=; the last 90 days by default.
func Usage(logger *slog.Logger, h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		author := cont.GetUser(ctx)
		log := loggerWith(logger, r, author)

		to := time.Now().UTC()
		if r.URL.Query().Get("to") != "" {
			t, err := request.GetDate(r, "to")
			if err != nil {
				web.FailCode(w, r, log, 400, 400, "Invalid parameter", err)
				return
			}
			// a bare date includes the whole day
			if len(r.URL.Query().Get("to")) == 10 {
				t = t.Add(24*time.Hour - time.Millisecond)
			}
			to = t
		}
		from := to.Add(-defaultUsagePeriod)
		if r.URL.Query().Get("from") != "" {
			f, err := request.GetDate(r, "from")
			if err != nil {
				web.FailCode(w, r, log, 400, 400, "Invalid parameter", err)
				return
			}
			from = f
		}

		data, err := h.GetAIUsage(ctx, author, from, to)
		if err != nil {
			web.Fail(w, r, log, 0, "Failed to read ai usage", err)
			return
		}
		web.OK(w, r, log.With(slog.Time("from", from), slog.Time("to", to)), "ai usage", data)
	}
}
