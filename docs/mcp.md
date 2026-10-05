# MCP server

evsys-back exposes the EVSys data to MCP clients (Claude, Claude Code, Claude
Desktop, any client speaking the Model Context Protocol) at

```
{public_url}/api/v1/mcp
```

It serves two kinds of work:

- **Technical analysis**: what is offline or faulted, what charge points
  reported (OCPP traffic, connector errors), why a session charged slowly
  (meter values: voltage, current drawn, current offered, assigned amperage).
- **Workload analysis**: the figures of the web statistics and reports pages -
  energy per month/user/charger/hour, power timelines, uptime, site
  concurrency against the load balancer's limits.

Every tool is read-only. A connection acts as the admin or operator who
approved it, under the same access rules the REST API applies to that user.

## Enabling it

```yaml
mcp:
  enabled: true
  public_url: "https://wattbrews.me"        # origin MCP clients reach this backend at
  frontend_url: "https://wattbrews.me"      # evsys-front origin, hosts the consent page
  access_token_ttl: 60                       # minutes
  refresh_token_ttl: 30                      # days
  request_timeout: 60                        # seconds per MCP request
```

Deployment sets `MCP_ENABLED`, `MCP_PUBLIC_URL` and `MCP_FRONTEND_URL` as
GitHub Actions variables; the token lifetimes and timeout keep their defaults
unless added to `back.yml`.

On start with MongoDB, the service creates the indexes of its two collections
(see [Storage](#storage)).

### Reverse proxy (required change)

The OAuth endpoints and the MCP endpoint live under `/api/v1`, which the proxy
already forwards. Discovery does not: an issuer with a path has its metadata at
the site root (RFC 8414), and clients try
`/.well-known/oauth-authorization-server/api/v1/oauth` first. On wattbrews.me
that path currently reaches the evsys-front SPA, which answers `200` with
`index.html` - and MCP client SDKs treat a 200 that is not JSON as a failure
instead of moving on to the next candidate URL. Forward the OAuth well-known
paths to the backend:

```nginx
# <port>: the backend, as for /api/; ^~ keeps regex locations (static files) from winning
location ^~ /.well-known/oauth-authorization-server { proxy_pass http://127.0.0.1:<port>; }
location ^~ /.well-known/oauth-protected-resource   { proxy_pass http://127.0.0.1:<port>; }
location ^~ /.well-known/openid-configuration/api/  { proxy_pass http://127.0.0.1:<port>; }
```

Check with:

```bash
curl -s https://wattbrews.me/.well-known/oauth-authorization-server/api/v1/oauth | head -c 80
# {"authorization_endpoint":"https://wattbrews.me/api/v1/oauth/authorize",...
```

Registration and authorization are unauthenticated, and at most 1000
requests can be pending at once, so rate-limit them per client address in the
proxy; nothing in the backend does:

```nginx
limit_req_zone $binary_remote_addr zone=oauth:1m rate=10r/m;
location ^~ /api/v1/oauth/register  { limit_req zone=oauth burst=5; proxy_pass http://127.0.0.1:<port>; }
location ^~ /api/v1/oauth/authorize { limit_req zone=oauth burst=5; proxy_pass http://127.0.0.1:<port>; }
```

Also allow `request_timeout` on `/api/v1/mcp` in the proxy: reports over a
long period can take tens of seconds. The MCP endpoint answers plain JSON, not
event streams, so no buffering settings are needed.

The backend listens on 127.0.0.1 behind the proxy. The MCP SDK's DNS
rebinding protection rejects exactly that combination (loopback address,
public Host header), so it is disabled; every request is authenticated by
bearer token instead.

## Connecting a client

**Claude (claude.ai, Desktop)**: Settings → Connectors → Add custom connector,
URL `https://wattbrews.me/api/v1/mcp`. Claude registers itself, opens the
consent page, and connects.

**Claude Code**:

```bash
claude mcp add --transport http evsys https://wattbrews.me/api/v1/mcp
```

then `/mcp` in a session to authenticate.

Either way the browser lands on the evsys-front consent page. Sign in if
needed, check the application name and where it returns to, and click Allow.

## Authorization

The backend is both the OAuth 2.1 authorization server and the protected
resource. Users never enter credentials into the OAuth server: the consent page
in evsys-front signs them in the usual way (Google, e-mail link or local
account) and reports the decision with the user's regular API token.

```
MCP client                  evsys-back                         evsys-front (browser)
    |  POST /api/v1/mcp            |                                    |
    |----------------------------->|  401, WWW-Authenticate:            |
    |<-----------------------------|  resource_metadata="..."           |
    |  GET  protected-resource     |                                    |
    |  GET  .well-known/oauth-authorization-server                     |
    |  POST /api/v1/oauth/register |  (dynamic client registration)     |
    |  open browser: GET /api/v1/oauth/authorize?...&code_challenge=... |
    |                              |-- 302 -> /oauth/authorize?request=ID
    |                              |<-- GET  /api/v1/oauth/requests/ID  | (user's API token)
    |                              |<-- POST /api/v1/oauth/requests/ID/approve
    |                              |--> {redirect_url: client?code=..}  |
    |<------------------------------------------------- browser redirect|
    |  POST /api/v1/oauth/token (code + code_verifier)                  |
    |<-- access_token, refresh_token                                    |
    |  POST /api/v1/mcp  Authorization: Bearer <access_token>           |
```

| Property | Value |
|---|---|
| Issuer | `{public_url}/api/v1/oauth` |
| Resource (token audience) | `{public_url}/api/v1/mcp` |
| Scope | `evsys:read` (the only one) |
| Grants | `authorization_code` with PKCE S256 (required), `refresh_token` |
| Client registration | open, RFC 7591; public clients (`none`) or `client_secret_post`/`client_secret_basic` |
| Redirect URIs | `https`, `http` on loopback (any port matches, RFC 8252), private-use schemes of desktop apps |
| Access token | opaque, 60 min, prefix `evsys_at_` |
| Refresh token | opaque, 30 days, rotated on every use, prefix `evsys_rt_` |

Rules worth knowing:

- **Client names are not verified**: registration is open, so anyone can
  register a client called "Claude" and send an admin the consent link. The
  consent page warns, naming the destination host, whenever the code would go
  anywhere other than claude.ai / claude.com or a local listener
  (`known_client: false`).
- **Who can connect**: admins and operators. The consent page tells other
  users they cannot, and the backend refuses their approval.
- **Role changes apply at once**: every MCP request reloads the user. Demote an
  admin and their connections stop working on the next call, not at token
  expiry; their refresh tokens are refused too.
- **Tokens are stored as SHA-256 hashes** only, so the database never holds a
  usable token. MCP tokens are not accepted by the REST API and REST API tokens
  are not accepted by the MCP endpoint.
- **Revocation** (`POST /api/v1/oauth/revoke`, RFC 7009) ends the whole grant:
  revoking either token of a connection invalidates both.
- **Pending requests and codes are in memory**: a request lives 10 minutes and
  a code 5. A restart in between only means clicking Connect again. At most
  1000 requests can be pending.

### Endpoints

Public:

| Method | Path | |
|---|---|---|
| GET | `/api/v1/oauth/protected-resource` | RFC 9728 metadata (announced in 401 responses) |
| GET | `/api/v1/oauth/.well-known/oauth-authorization-server` | RFC 8414 metadata |
| GET | `/api/v1/oauth/.well-known/openid-configuration` | same document, for clients that look there |
| GET | `/.well-known/oauth-protected-resource[/...]` | same document, served once the proxy forwards it (see Reverse proxy) |
| GET | `/.well-known/oauth-authorization-server[/...]` | same document, served once the proxy forwards it (see Reverse proxy) |
| POST | `/api/v1/oauth/register` | dynamic client registration |
| GET | `/api/v1/oauth/authorize` | starts authorization, redirects to the consent page |
| POST | `/api/v1/oauth/token` | code and refresh token grants (form-encoded) |
| POST | `/api/v1/oauth/revoke` | token revocation (form-encoded) |

User token (called by evsys-front):

| Method | Path | |
|---|---|---|
| GET | `/api/v1/oauth/requests/{id}` | describes the pending request: client name, redirect host, expiry, `known_client`, `can_approve` |
| POST | `/api/v1/oauth/requests/{id}/approve` | `{redirect_url}` carrying the code; 403 for users who may not connect |
| POST | `/api/v1/oauth/requests/{id}/deny` | `{redirect_url}` carrying `error=access_denied` |

A request is consumed by the decision: a second call returns 404.

## Tools

All inputs are optional unless marked. `from`/`to` take RFC 3339 or a date
(`2026-10-01`); a bare `to` date includes the whole day; times are UTC. Units in
results: Wh, W, A, V, cents of EUR.

| Tool | What it returns |
|---|---|
| `system_overview` | Charge points online/offline, connectors by status, current problems, sessions charging now with their power. The starting point. |
| `list_charge_points` | Charge points with connectors, firmware, last status and error, assigned current limit and the charger's answer to it. `search`, `problems_only`. |
| `get_charge_point` | One charge point in full. `charge_point_id` (required). |
| `list_locations` | Sites with power settings and their charge points. Needs access level 10. |
| `list_active_transactions` | Running sessions of all users, with the latest meter reading. |
| `search_transactions` | Finished sessions in a period (default 30 days), by charge point, user, RFID tag or payment failure; with totals. `limit` default 100, max 1000. |
| `get_transaction` | One session with tariff, payment and the meter value series, evenly sampled to `max_meter_points` (default 120, max 2000). `transaction_id` (required). |
| `read_log` | One log, newest first: `sys` (OCPP traffic), `back` (backend), `pay` (payments), `errors` (connector errors). Filters: period, `charge_point_id`, `search` (substring), `category` (OCPP feature / category / error code), `level`. `limit` default 100, max 1000. `log` (required). |
| `error_summary` | Connector errors counted per charge point, connector and error code, with first and last occurrence. Default 7 days. |
| `energy_report` | Energy and sessions per `month`, `user`, `charger` or `hour` for one user group (default `default`) - the statistics page. Default 30 days. `group_by` (required). |
| `power_report` | Power per `charger`, `session`, or as an `hour`/`day` timeline of fleet load. Default 7 days. |
| `station_uptime` | Connected time per enabled charge point. Default 7 days. |
| `station_status` | Whether each enabled charge point is connected now, and since when. |
| `site_concurrency` | Per location: overlapping sessions, assigned vs measured peak. Default 7 days. |
| `list_users` | Accounts with role, group, last activity. Admins only. |
| `list_user_tags` | RFID tags with owner and note. |
| `payment_retry_queue` | Failed payments waiting for retry. |
| `webhook_status` | Webhook subscriber counters and recent failed deliveries. |

What the tools never return: passwords, API tokens, card tokens
(`identifier`, `merchant_cof_txnid`) and webhook subscriber secrets.

A result above 400 KB is refused with a request to narrow the query: an MCP
client hands the result to the model whole, and one that overflows the
context is worse than an error.

The server is stateless (no MCP sessions): each call stands alone, so a
backend restart does not break a connected client.

## Storage

Two collections, owned by this service:

- `oauth_clients` - registered clients. Unique index on `client_id`.
- `oauth_tokens` - `{hash, kind, grant_id, client_id, username, scope, resource,
  expires_at}`. Unique index on `hash`, index on `grant_id`, and a TTL index on
  `expires_at` that lets MongoDB delete expired tokens.

To disconnect everyone at once, drop `oauth_tokens`; to disconnect one user,
delete their documents (`{username: "..."}`).

## Code map

| Path | Role |
|---|---|
| `internal/mcpserver/` | MCP server: tools, input parsing, output views, bearer auth |
| `impl/oauth/` | Authorization server: registration, consent, codes, tokens |
| `internal/api/handlers/oauth/` | HTTP handlers of the OAuth endpoints |
| `impl/core/inspect.go` | Read-only data access for the tools, power users only |
| `impl/database/oauth.go` | OAuth storage |
| evsys-front `components/oauth-consent/` | Consent page |
