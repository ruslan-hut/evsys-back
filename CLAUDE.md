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

## Configuration

Two config files:
- `config.yml` - Local development (hardcoded values, mongo disabled)
- `back.yml` - Deployment template with `${ENV_VAR}` placeholders

Key config sections: `listen` (server), `mongo` (database), `central_system` (external API), `firebase_key` (optional auth).

## Local Development

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

## Deployment

CI/CD via GitHub Actions (`.github/workflows/deploy.yml`):
- Push to master triggers build and deploy
- Replaces env placeholders in `back.yml`
- Deploys to `/usr/local/bin/evsys-back`
- Restarts systemd service `evsys-back.service`

## User Preferences

- **Do not run build/test commands** - The user will build and test by themselves
