# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run Commands

```bash
# Download dependencies
go mod download

# Run locally (mock DB, no MongoDB required)
go run ./main.go -conf config.yml -log ./logs

# Build binary
go build -v -o evsys-back

# Run tests
go test ./...

# Format and vet
go fmt ./...
go vet ./...
```

## Project Overview

Go 1.24 backend for EV charging management. Exposes REST API (`/api/v1`) and WebSocket (`/ws`) for managing users, locations, charge points, transactions, and payments.

## Architecture

```
main.go                     Entry point: parses flags, loads config, initializes components
    ↓
config/                     YAML configuration loader (cleanenv)
    ↓
impl/                       Core implementations
├── core/                   Business logic orchestrator - aggregates all services
├── database/               MongoDB persistence (mongo.go)
├── database-mock/          In-memory mock for local dev (returns stubs)
├── authenticator/          Token-based auth + optional Firebase
├── redsys/                 Redsys payment gateway client (MIT payments, preauth, refunds)
├── brevo/                  Brevo (Sendinblue) transactional email client
├── mail/                   Scheduled report-mail service (daily/weekly/monthly)
├── reports/                Statistics generation
├── status-reader/          Transaction state management
└── central-system/         External API proxy
    ↓
internal/api/
├── http/server.go          Chi router, WebSocket pool, middleware stack
├── handlers/               REST endpoints (users/, locations/, transactions/, payments/, report/)
├── middleware/             authenticate (token validation), timeout (5s)
└── lib/                    Utilities (logger, validate, api/response, api/request)
    ↓
entity/                     Domain models and DTOs (26 files)
```

## Key Patterns

**Repository Pattern**: All handlers depend on interfaces defined in `impl/core/`, not concrete implementations. Storage is switchable between MongoDB and in-memory mock via config.

**WebSocket Pool**: `internal/api/http/server.go` manages concurrent WebSocket connections with channels. Three subscription types: broadcast, log-event, charge-point-event.

**Structured Logging**: Uses `log/slog`. Environment determines output format:
- local: text to stdout (DEBUG)
- dev: JSON to file (DEBUG)
- prod: JSON to file (INFO)

**Secret Masking**: `internal/lib/sl/sl.go` provides `sl.Secret()` that shows only first 5 characters in logs.

**Redsys Payment Integration**: `impl/redsys/` provides the Redsys REST API client behind an adapter pattern (`core.RedsysClient` interface). Supports:
- **Preauthorization/Capture** — two-phase payment: hold amount, then capture later
- **Direct MIT Payment** — Merchant Initiated Transaction using stored card tokens (PSD2 exempt)
- **Refunds** — full (by transaction) or partial (by order)
- **Webhook notifications** — async Redsys callbacks via `POST /api/v1/payment/notify` (unauthenticated)
- **Per-order locking** — `sync.Map`-based mutex in Core for concurrent payment safety
- **Payment method fallback** — auto-switches to alternative method when FailCount > 0
- **DisablePayment mode** — bypasses Redsys calls for testing (`redsys.disable_payment: true`)

**Mail Reports (Brevo)**: `impl/brevo/` posts transactional emails to the Brevo HTTP API; `impl/mail/` runs a goroutine that wakes daily at 06:00 UTC and dispatches per-charger statistics to admin-managed subscribers (daily / weekly on Mon / monthly on the 1st). See [docs/brevo-setup.md](docs/brevo-setup.md) for provider setup.

## Configuration

Two config files:
- `config.yml` - Local development (hardcoded values, mongo disabled)
- `back.yml` - Deployment template with `${ENV_VAR}` placeholders

Key config sections: `listen` (server), `mongo` (database), `central_system` (external API), `firebase_key` (optional auth), `redsys` (payment gateway), `brevo` (mail provider).

### Redsys Payment Config

```yaml
redsys:
  enabled: false              # Enable Redsys integration
  disable_payment: false      # Bypass Redsys API calls (test mode — marks transactions as paid)
  merchant_code: ""           # Redsys merchant code
  terminal: "001"             # Terminal ID
  secret_key: ""              # Base64-encoded secret for signature generation
  rest_api_url: "https://sis-t.redsys.es:25443/sis/rest/trataPeticionREST"  # Test endpoint
  currency: "978"             # ISO 4217 currency code (978 = EUR)
  api_key: ""                 # API key for service-to-service auth (central system → payment endpoints)
```

### Brevo Mail Config

```yaml
brevo:
  enabled: false                                   # Enable scheduled report emails
  api_key: ""                                      # Brevo v3 API key (xkeysib-...)
  sender_name: "EVSys Reports"                     # From-name shown in inbox
  sender_email: "noreply@example.com"              # Verified Brevo sender address
  api_url: "https://api.brevo.com/v3/smtp/email"   # Brevo transactional endpoint
```

See [docs/brevo-setup.md](docs/brevo-setup.md) for full provider walk-through.

1. With mock DB (no external dependencies):
   ```bash
   go run ./main.go -conf config.yml -log ./logs
   ```
   Test token: `12345678901234567890123456789000` (user: test, access level 1)

2. With MongoDB: Set `mongo.enabled: true` in config.yml and provide connection details.

## Adding New Features

**New REST endpoint:**
1. Create handler in `internal/api/handlers/{domain}/`
2. Register route in `internal/api/http/server.go`
3. Define request/response entities in `entity/`

**New database operation:**
1. Add method to repository interface in `impl/core/`
2. Implement in both `impl/database/mongo.go` and `impl/database-mock/mock-db.go`

**New Redsys transaction type:**
1. Add constant in `impl/redsys/client.go`
2. Add method to `Client` (use `sendRequest`/`performMITTransaction`/`performSimpleTransaction`)
3. Add corresponding method to `core.RedsysClient` interface + request type in `impl/core/core.go`
4. Add adapter method in `impl/redsys/adapter.go`

## Payment API Endpoints

User-authenticated (require user token):
- `GET /api/v1/payment/methods` — List user's payment methods
- `POST /api/v1/payment/save` — Save a payment method
- `POST /api/v1/payment/update` — Update a payment method
- `POST /api/v1/payment/delete` — Delete a payment method
- `POST /api/v1/payment/order` — Create a payment order

API-key-authenticated (service-to-service, `Authorization: Bearer {api_key}`):
- `GET /api/v1/payment/pay/{transactionId}` — Initiate direct MIT payment
- `GET /api/v1/payment/return/{transactionId}` — Full refund for transaction
- `POST /api/v1/payment/return/order/{orderId}` — Partial refund (JSON body: `{"amount": N}`)

Unauthenticated (Redsys webhook):
- `POST /api/v1/payment/notify` — Redsys async payment notification

## Mail Subscription API Endpoints

Admin/operator only (`RequirePowerUser`):
- `GET    /api/v1/mail/subscriptions`               — List all subscriptions
- `POST   /api/v1/mail/subscriptions`               — Create subscription `{email, period, user_group, enabled}`
- `PUT    /api/v1/mail/subscriptions/{id}`          — Update subscription
- `DELETE /api/v1/mail/subscriptions/{id}`          — Delete subscription
- `POST   /api/v1/mail/subscriptions/{id}/send-now` — Trigger an immediate report email
- `POST   /api/v1/mail/test`                        — Send a minimal diagnostic email `{email}` to verify Brevo

`period` is one of `daily | weekly | monthly`. `user_group` matches the same value the frontend statistic page sends as the `group` query param (e.g. `default`, `office`).

## Deployment

CI/CD via GitHub Actions (`.github/workflows/deploy.yml`):
- Push to master triggers build and deploy
- Replaces env placeholders in `back.yml`
- Deploys to `/usr/local/bin/evsys-back`
- Restarts systemd service `evsys-back.service`

## User Preferences

- **Always verify builds on significant code changes** - Run `go build ./...` (and `go vet ./...` when warranted) after non-trivial backend edits. For the Angular frontend at `~/projects/evsys-front`, run `npm run build` after non-trivial changes. Skip only for purely trivial edits (typos, comments, single-line tweaks).
