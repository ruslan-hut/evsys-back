// Package mcpserver exposes the EVSys data to MCP clients such as Claude:
// charge points, transactions, logs and the workload reports of the web UI.
// Every tool is read-only and acts as the admin or operator who approved the
// connection, with the same access rules the REST API applies to them.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "evsys"
	serverVersion = "1.0.0"
	userKey       = "user"

	// maxResultBytes bounds one tool result. A client hands the result to the
	// model whole, and a result that does not fit its context is worse than an
	// error asking for a narrower query.
	maxResultBytes = 400_000
)

const instructions = `EVSys is an OCPP charging network: charge points grouped into locations, each with connectors; users identify with RFID tags (id_tag) to run charging sessions (transactions). The OCPP central system (evsys) writes the data; this server reads it.

Units: energy in Wh, power in W, currents in A, voltages in V, money in cents of EUR. Times are UTC, RFC 3339. Inputs "from"/"to" accept RFC 3339 or YYYY-MM-DD; a bare "to" date includes the whole day.

Where to start:
- system_overview for the current state: what is online, faulted, charging.
- Technical analysis: get_charge_point, read_log (sys = OCPP messages from charge points, back = this backend, pay = payments, errors = connector errors), error_summary, station_uptime, station_status, get_transaction (meter values: power, voltage, current, current offered by the charger).
- Workload analysis: energy_report, power_report, site_concurrency, search_transactions.

Connector status is the last one the charge point reported; when is_online is false it may be stale. Transaction power_limit is the amperage the load balancer assigned to the session, not a power figure.`

// Core is the data the tools read; implemented by impl/core.
type Core interface {
	InspectLocations(ctx context.Context, author *entity.User) ([]*entity.Location, error)
	InspectChargePoints(ctx context.Context, author *entity.User, search string) ([]*entity.ChargePoint, error)
	InspectChargePoint(ctx context.Context, author *entity.User, id string) (*entity.ChargePoint, error)
	InspectActiveTransactions(ctx context.Context, author *entity.User) ([]*entity.ChargeState, error)
	InspectTransactions(ctx context.Context, author *entity.User, filter *entity.TransactionFilter) ([]*entity.Transaction, error)
	InspectTransaction(ctx context.Context, author *entity.User, id int) (*entity.ChargeState, error)
	InspectLog(ctx context.Context, author *entity.User, name string, filter *entity.LogFilter) (any, error)
	InspectErrorSummary(ctx context.Context, author *entity.User, from, to time.Time, chargePointId string) ([]*entity.ErrorSummary, error)

	MonthlyStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]any, error)
	UsersStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]any, error)
	ChargerStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]any, error)
	ExportStats(ctx context.Context, user *entity.User, from, to time.Time, userGroup string) ([]any, error)
	PowerStatsReport(ctx context.Context, user *entity.User, from, to time.Time, chargePointId, userGroup, groupBy string) ([]*entity.PowerStats, error)
	StationUptimeReport(ctx context.Context, user *entity.User, from, to time.Time, chargePointId string) ([]*entity.StationUptime, error)
	StationStatusReport(ctx context.Context, user *entity.User, chargePointId string) ([]*entity.StationStatus, error)
	SiteConcurrencyReport(ctx context.Context, user *entity.User, from, to time.Time, locationId string, minSessions, maxSegments int) ([]*entity.SiteConcurrency, error)

	GetUsers(ctx context.Context, user *entity.User) ([]*entity.User, error)
	ListUserTags(ctx context.Context, author *entity.User) ([]*entity.UserTag, error)
	GetPaymentRetryQueue(ctx context.Context, author *entity.User) ([]*entity.PaymentRetryView, error)
	GetWebhookHealth(ctx context.Context, author *entity.User) ([]*entity.WebhookHealthView, error)
	ListWebhookFailures(ctx context.Context, author *entity.User) ([]*entity.WebhookDeliveryView, error)
}

// TokenVerifier resolves a bearer token to the user it acts for; implemented
// by impl/oauth. Errors wrapping entity.ErrInvalidToken mean the token does not
// grant access.
type TokenVerifier interface {
	VerifyAccessToken(ctx context.Context, token string) (*entity.User, *entity.OAuthToken, error)
}

type tools struct {
	core Core
	log  *slog.Logger
	now  func() time.Time
}

// NewServer builds the MCP server with all tools registered.
func NewServer(log *slog.Logger, core Core) *mcp.Server {
	t := &tools{core: core, log: log.With(sl.Module("mcp.tools")), now: time.Now}
	server := mcp.NewServer(&mcp.Implementation{Name: serverName, Title: "EVSys", Version: serverVersion}, &mcp.ServerOptions{
		Instructions: instructions,
	})
	t.register(server)
	return server
}

// NewHandler serves the MCP streamable HTTP transport, behind bearer token
// authentication. The server is stateless: every tool call stands alone, so
// a restart of the backend does not break connected clients.
// resourceMetadataUrl is announced in 401 responses for OAuth discovery.
func NewHandler(log *slog.Logger, core Core, verifier TokenVerifier, resourceMetadataUrl string) http.Handler {
	server := NewServer(log, core)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		// The backend listens on 127.0.0.1 behind a reverse proxy, so every
		// request arrives on a loopback address carrying the public Host
		// header - exactly what the DNS rebinding check rejects. Requests are
		// authenticated by bearer token, which that check is no substitute for.
		DisableLocalhostProtection: true,
	})
	verify := func(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		user, stored, err := verifier.VerifyAccessToken(ctx, token)
		if err != nil {
			if errors.Is(err, entity.ErrInvalidToken) {
				return nil, fmt.Errorf("%w: %v", auth.ErrInvalidToken, err)
			}
			log.Error("verify mcp token", sl.Err(err))
			return nil, err
		}
		return &auth.TokenInfo{
			Scopes:     []string{stored.Scope},
			Expiration: stored.ExpiresAt,
			UserID:     user.Username,
			Extra:      map[string]any{userKey: user},
		}, nil
	}
	// The SDK writes the URL into WWW-Authenticate as is; RFC 9728 wants the
	// auth-param quoted, and a URL is not a valid bare token.
	return auth.RequireBearerToken(verify, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: `"` + resourceMetadataUrl + `"`,
		Scopes:              []string{entity.OAuthScope},
	})(handler)
}

// userFrom returns the user a tool call acts for.
func userFrom(req *mcp.CallToolRequest) (*entity.User, error) {
	if req == nil || req.Extra == nil || req.Extra.TokenInfo == nil {
		return nil, fmt.Errorf("not authenticated")
	}
	user, ok := req.Extra.TokenInfo.Extra[userKey].(*entity.User)
	if !ok || user == nil {
		return nil, fmt.Errorf("not authenticated")
	}
	return user, nil
}

// handlerFor adapts a tool function to the SDK: it resolves the user, logs the
// call and serializes the result as one JSON text block.
func handlerFor[In any](t *tools, name string, fn func(ctx context.Context, user *entity.User, in In) (any, error)) mcp.ToolHandlerFor[In, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (_ *mcp.CallToolResult, _ any, err error) {
		// The SDK runs handlers on their own goroutine without recovering, so
		// a panic here would take the whole backend down, REST API included.
		defer func() {
			if r := recover(); r != nil {
				t.log.With(slog.String("tool", name), slog.Any("panic", r), slog.String("stack", string(debug.Stack()))).Error("mcp tool panicked")
				err = fmt.Errorf("internal error in %s", name)
			}
		}()
		user, err := userFrom(req)
		if err != nil {
			return nil, nil, err
		}
		started := time.Now()
		log := t.log.With(slog.String("tool", name), slog.String("user", user.Username))
		out, err := fn(ctx, user, in)
		if err != nil {
			log.With(sl.Err(err)).Warn("mcp tool failed")
			return nil, nil, err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return nil, nil, fmt.Errorf("encoding result: %w", err)
		}
		log.With(
			slog.Int("bytes", len(body)),
			slog.Duration("duration", time.Since(started)),
		).Debug("mcp tool call")
		if len(body) > maxResultBytes {
			return nil, nil, fmt.Errorf("result too large (%d KB): narrow the period, add a filter or lower the limit", len(body)/1024)
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(body)}}}, nil, nil
	}
}

// addTool registers a read-only tool.
func addTool[In any](t *tools, server *mcp.Server, name, title, description string, fn func(ctx context.Context, user *entity.User, in In) (any, error)) {
	closedWorld := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		Annotations: &mcp.ToolAnnotations{
			Title:          title,
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  &closedWorld,
		},
	}, handlerFor(t, name, fn))
}
