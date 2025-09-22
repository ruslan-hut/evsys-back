# evsys-back

Backend service for an EV charging management system. It exposes a REST API and a WebSocket endpoint to manage users, locations, charge points, transactions, payments, and reports. Storage uses MongoDB when enabled; a mock in-memory DB can be used for local development. Optional integrations include Firebase (for authentication) and a Central System API.

## Overview
- Language/stack: Go 1.24 (module: `evsys-back`)
- HTTP framework: chi (router), gorilla/websocket
- Storage: MongoDB (configurable), in-memory mock for local/dev
- Auth: Optional Firebase integration; custom token-based middleware
- Config: YAML file, with support for env variable substitution in the provided `back.yml`
- Entry point: `main.go` → starts HTTP server defined in `internal/api/http/server.go`

Key features:
- REST API under `/api/v1` for users, locations, charge points, transactions, payments, and reports
- WebSocket endpoint at `/ws` for real-time updates (transactions/logs)
- Central System command proxying (optional)
- Structured logging with slog

## Requirements
- Go 1.24.7 toolchain (see `go.mod`: `toolchain go1.24.7`)
- MongoDB instance (if `mongo.enabled: true`)
- Optional Firebase Admin credentials JSON (if `firebase_key` provided)
- Network access to the Central System API if that integration is enabled

## Configuration
The application reads a YAML config file.

- Default path: `config.yml` (can be overridden with `-conf` flag)
- Sample local config: `config.yml`
- Deployment template with env placeholders: `back.yml`

Config schema (see `config/config.go`):
- env: string (default: local)
- time_zone: string (default: UTC)
- log_records: int64 (default: 0)
- firebase_key: string (path to Firebase service account JSON; default: empty/disabled)
- listen:
  - type: string (default: port)
  - bind_ip: string (default: 0.0.0.0)
  - port: string (default: 5000)
  - tls_enabled: bool (default: false)
  - cert_file: string (default: empty)
  - key_file: string (default: empty)
- central_system:
  - enabled: bool (default: false)
  - url: string
  - token: string
- mongo:
  - enabled: bool (default: false)
  - host: string (default: 127.0.0.1)
  - port: string (default: 27017)
  - user: string (default: admin)
  - password: string (default: pass)
  - database: string

Flags:
- `-conf` path to config file (default `config.yml`)
- `-log` directory for log files (default `/var/log/wattbrews`)

### Environment variables (deployment)
The CI/CD workflow uses `back.yml` and replaces placeholders with GitHub variables/secrets. If you use `back.yml`, the following env vars are expected:
- TIME_ZONE
- FIREBASE_KEY (secret: raw JSON content or path depending on your deployment; in CI it’s a secret string)
- PORT
- TLS_ENABLED (true/false)
- CERT_FILE
- KEY_FILE
- CENTRAL_SYSTEM_URL (secret)
- CENTRAL_SYSTEM_TOKEN (secret)
- MONGO_HOST
- MONGO_PORT
- MONGO_USER (secret)
- MONGO_PASS (secret)
- MONGO_DB

Note: When running locally with `config.yml`, you can hardcode values instead of using env substitutions.

## Running locally
1. Ensure Go 1.24.7 toolchain is available.
2. Optionally start MongoDB if you want persistence, then set `mongo.enabled: true` in `config.yml` and configure connection.
3. If you don’t enable MongoDB, a mock in-memory DB will be used.
4. If you want Firebase auth, provide a service account JSON and set `firebase_key` in the config.

Commands:
- Download deps: `go mod download`
- Run: `go run ./main.go -conf config.yml -log ./logs`  
  The server will bind to `listen.bind_ip:listen.port`. TLS can be enabled via config if desired.
- Build: `go build -v -o evsys-back`

## REST and WebSocket endpoints
Base path for REST: `/api/v1`

Auth-required group (middleware `authenticate`):
- GET `/api/v1/locations` — list locations
- GET `/api/v1/chp` and `/api/v1/chp/{search}` — list charge points
- GET `/api/v1/point/{id}` — read charge point
- POST `/api/v1/point/{id}` — update/save charge point
- GET `/api/v1/users/info/{name}` — user info
- GET `/api/v1/users/list` — list users
- POST `/api/v1/csc` — central system command
- GET `/api/v1/transactions/active` — active transactions
- GET `/api/v1/transactions/list` and `/api/v1/transactions/list/{period}` — list transactions
- GET `/api/v1/transactions/info/{id}` — transaction details
- GET `/api/v1/payment/methods` — list payment methods
- POST `/api/v1/payment/save` — save payment method
- POST `/api/v1/payment/update` — update payment method
- POST `/api/v1/payment/delete` — delete payment method
- POST `/api/v1/payment/order` — create payment order
- GET `/api/v1/report/month` — monthly statistics
- GET `/api/v1/report/user` — user statistics
- GET `/api/v1/report/charger` — charger statistics
- GET `/api/v1/log/{name}` — read log

Public (no auth):
- GET `/api/v1/config/{name}` — read config by name
- POST `/api/v1/users/authenticate` — authenticate user
- POST `/api/v1/users/register` — register user

WebSocket:
- `GET /ws` — real-time updates; subscription controlled by messages over the socket. CORS origin check is open (allows all).

Note: Detailed request/response schemas are defined in the `entity/` package and handler implementations under `internal/api/handlers/...`.

## Project structure
- `main.go` — application entry point
- `config/` — configuration loader
- `internal/api/http/` — HTTP server, router, WebSocket, middleware setup
- `internal/api/handlers/` — REST handlers (users, locations, payments, transactions, reports, central-system)
- `impl/` — implementations for core domain, database (mongo/mock), reports, central system, authenticator, status reader
- `entity/` — domain entities and DTOs
- `internal/lib/` — common utilities (logging, validation, API helpers, time, etc.)
- `config.yml` — local config example
- `back.yml` — deployment config template using env vars
- `.github/workflows/deploy.yml` — GitHub Actions build/deploy workflow

## Scripts and automation
There is no Makefile. Use standard Go commands:
- Run: `go run ./main.go -conf <config-file> [-log <log-dir>]`
- Build: `go build -v -o evsys-back`
- Test: `go test ./...`

CI/CD (GitHub Actions):
- On push to `master`, the workflow:
  - Replaces placeholders in `back.yml` with repo variables/secrets
  - Copies `back.yml` to remote `/etc/conf/`
  - Builds the binary (`evsys-back`)
  - Copies the binary to `/usr/local/bin/`
  - Restarts a systemd service `evsys-back.service`

## Logging
- Uses Go slog; logs are configured via `internal/lib/logger`.
- `-log` flag points to the log directory. The number of stored log records can be influenced via `log_records` in config (see implementation for exact behavior).

## License
This project is licensed under the MIT License. See the LICENSE file for details.

## Security
- Keep secrets (Firebase key, Central System token, DB credentials) out of source control. Use environment variables and secret stores in CI/CD.
