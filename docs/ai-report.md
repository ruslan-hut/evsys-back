# AI analysis

A weekly report on the health of the WattBrews network, written by an OpenAI model from
the system's own data and emailed to the subscribers of the `ai_analysis`
report, each in their language (Spanish by default, or English).

## How a run works

1. **Snapshot** (`impl/aireport/snapshot.go`) - the service reads the period
   and the period before it straight from MongoDB: the charge points now
   (online, offline, Faulted or Unavailable connectors), uptime per station,
   connector errors (`errors_log`), the `sys_log`, `back_log` and `payment_log`
   journals, finished sessions, the payment retry queue and the webhook
   outbox. Logs are reduced to counts and to the *shapes* of notable records:
   numbers, addresses, times, user ids and emails are replaced
   (`template.go`), so a week of journal fits in about 50-150 KB and no
   personal data reaches OpenAI. The JSON is capped at 160 KB; lists are cut
   and the cut is noted in the snapshot.
   Money: the system takes payments only in the app; terminal (bank card at
   the charger, tag source `CC`) and roaming (`OCPI`) sessions are settled
   outside it, so their amounts are unknown. The snapshot therefore estimates
   turnover as the energy delivered to clients - every session except those
   of users outside the `default` group ("Cliente"), i.e. internal `office`
   users - at the tariff price per kWh: the session's own payment plan, else
   the default plan (`payment_plans`, `is_default`). `by_channel` splits
   sessions into app / terminal / roaming / internal / other, and
   `app_billed_eur` keeps what the app actually charged.
2. **Analysis** - one request to the Responses API per language, with a strict
   JSON schema (`prompt.go`). The snapshot comes first and the language last,
   so the second language reads the snapshot from OpenAI's prompt cache.
   Responses are not stored at OpenAI (`store: false`).
3. **Email** (`render.go`) - a compact one-page report: title, locations,
   period and status; four figure tiles (completed sessions, energy,
   estimated turnover, connector fault reports, each against the previous
   period) computed from the snapshot, not by the model; the executive
   diagnosis (`diagnosis`: up to 5 rows of subject - a charge point or an area
   - and one sentence); the action plan (`actions`: priority `urgent` /
   `priority` / `planned` shown as P1 / P2 / P3, equipment, action) and the
   closing criterion (`closing_note`). Reports generated before this layout
   carry `summary` / `findings` / `trends` instead; their findings are shown
   in the same two tables (title as diagnosis, recommendation as action).

Every request, failed ones included, is stored in `ai_usage` with its tokens
and its cost at the list prices of `entity.AIModels()` (a model missing from
that list is priced as the dearest one). A request is refused once the spend
of the calendar month (UTC) reaches the budget in the settings; the request
that crosses it is not cut short. A request that fails without an answer
(timeout, network) records no tokens, though OpenAI may still bill it.

## Locations

A subscription may name `location_ids`; empty means the whole network. A run
groups its recipients by the set of locations they follow and builds one
snapshot per set (`ai_reports.scopes`, keyed by the sorted ids joined with
commas, `""` for the whole network), then one request per set and language.
A recipient only ever gets the analysis of their own set - another language
of it is the fallback, never another set.

A snapshot limited to locations holds their charge points, uptime, connector
errors, sessions and their `sys_log` records, plus central-system records
(`*`) that name one of their charge points. The backend and payment logs, the
payment retry queue and webhooks are network-wide and left out (noted in the
snapshot). The email subject and header name the locations.

`GET /api/v1/mail/locations` (admins and operators) lists every location
with its charge points for the picker.

## Schedule

The service checks once a minute. A run is due at `weekday`/`hour` in
`time_zone` (default Monday 07:00 Europe/Madrid, which is 06:00 UTC in winter
and 05:00 UTC in summer) and covers the `period_days` whole local days before
it. The slot is claimed atomically in `ai_settings.last_scheduled_run`, so a
restart or a second instance never sends twice.

- A slot missed by up to 6 hours (restart, outage) is still run - unless the
  settings were saved after it: enabling the analysis or moving its hour does
  not send at once.
- A run interrupted by a stop is closed as failed on the next start; if it had
  sent nothing, its slot is released and runs again within the window.
- A run that fails (model error, budget) is not retried automatically - start
  it from the admin page ("generate and send").
- A manual run in progress delays the scheduled one to a later tick.
- Nothing runs without an OpenAI key, without Brevo, or without an enabled
  subscriber.

## Configuration

The OpenAI key is in the service configuration only - never in the database:

```yaml
openai:
  api_key: ${OPENAI_API_KEY}   # GitHub secret OPENAI_API_KEY
  api_url: https://api.openai.com/v1
  timeout: 300                 # seconds per request
```

Everything else is edited by an admin in evsys-front and kept in
`ai_settings` (`_id: ai_report`): enabled, model, reasoning effort, output
token limit, system prompt (empty = built-in `DefaultSystemPrompt`), schedule,
period and monthly budget.

## Collections

| Collection | Content |
|---|---|
| `ai_settings` | one document, `_id: ai_report` |
| `ai_reports` | one per run: period, snapshot, answers per language, deliveries, cost |
| `ai_usage` | one per OpenAI request: tokens, cost, duration, error |
| `mail_subscriptions` | `report: ai_analysis` + `language` mark the subscribers |

## Mail subscriptions

`mail_subscriptions` carries two optional fields:

- `report` - `statistic` (default; also every document written before the
  field existed) or `ai_analysis`.
- `language` - `es` (default) or `en`; used by the AI analysis.
- `location_ids` - AI analysis only: the locations it covers; absent or
  empty is the whole network. On update, an absent field keeps the stored
  list and `[]` clears it.

An `ai_analysis` subscription has no `period` or `user_group` (both are stored
empty, so neither the statistic scheduler nor an older binary after a rollback
picks it up). An update that leaves `report` or `language` out keeps the stored
values; documents written before the fields existed are returned with
`statistic` / `es`.
`POST /mail/subscriptions/{id}/send-now` on an `ai_analysis` subscription sends
the latest report of the current period in the subscriber's language - as it
was generated, possibly days ago - and generates one when there is none yet;
it returns at once and the email follows.

## API (admins only)

All under `/api/v1/ai`, guarded by `RequireAdmin`, with a 120 s timeout.

| Method | Path | |
|---|---|---|
| GET | `/ai/settings` | settings + `has_api_key`, `mail_enabled`, `default_system_prompt`, `models` (with prices), `languages`, `next_run`, `month_spent_usd`, `recipients` |
| PUT | `/ai/settings` | partial update; absent fields stay; `system_prompt: ""` restores the default |
| POST | `/ai/test` | a minimal request to check key and model; returns its usage record |
| POST | `/ai/reports/run` | `{dry_run, language, email, send_all}` → 202 with the report, which goes on in the background; poll `GET /ai/reports/{id}` until `status` is `done` or `failed` |
| GET | `/ai/reports?limit=` | latest runs, without snapshots |
| GET | `/ai/reports/{id}` | one run with its snapshot |
| GET | `/ai/reports/{id}/preview?scope=&language=` | `{scope, language, subject, html}` of the email; `scope` is a key from the report's `scopes` |
| GET | `/ai/usage?from=&to=` | `totals`, `months`, `models` and `records`; default the last 90 days |

`run` takes optional `location_ids` (not with `send_all`, which follows the
subscriptions). Variants: `dry_run` collects the snapshot only (no model, no cost);
without `email` the report is generated in `language` and kept for preview;
with `email` it is also sent there; `send_all` generates the languages of the
subscribers and sends to all of them. One run at a time: a second start
answers 400 "an analysis is already running".
