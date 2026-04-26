# Brevo (Sendinblue) Mail Provider Setup

This backend uses [Brevo](https://www.brevo.com) (formerly Sendinblue) as the
transactional email provider for scheduled session-statistics reports
(daily / weekly / monthly). This document covers everything needed to enable
the integration end-to-end.

---

## 1. Create / sign in to a Brevo account

1. Go to <https://app.brevo.com/> and sign up or log in.
2. The free plan is sufficient for low volumes (300 emails/day at the time of
   writing). Upgrade plans only if you expect higher throughput.

---

## 2. Verify a sender address

Brevo refuses to deliver mail from unverified senders.

1. Open **Senders, Domains & Dedicated IPs → Senders**.
2. Click **Add a sender** and enter the address that will appear in the `From:`
   header (for example `reports@your-domain.com`).
3. Confirm the verification email Brevo sends to that address.

For best deliverability also verify the **domain** itself in
**Senders, Domains & Dedicated IPs → Domains**, which lets you publish the
SPF / DKIM / DMARC DNS records Brevo recommends. This step is optional but
strongly advised for any production deployment.

---

## 3. Generate an API key

1. Open **SMTP & API → API Keys**.
2. Click **Generate a new API key**.
3. Name it (e.g. `evsys-back-prod`) and copy the value — it starts with
   `xkeysib-...`. Store it somewhere safe; Brevo will not show it again.

The backend uses Brevo's **v3 transactional email REST endpoint**
(`POST https://api.brevo.com/v3/smtp/email`) with the `api-key` header.
SMTP credentials are **not** required.

---

## 4. Configure the backend

### 4.1 Local development (`config.yml`)

```yaml
brevo:
  enabled: true
  api_key: "xkeysib-...your-key..."
  sender_name: "EVSys Reports"
  sender_email: "reports@your-domain.com"
  api_url: "https://api.brevo.com/v3/smtp/email"
```

Set `enabled: false` to disable the integration entirely (the scheduler is not
started, and the admin UI's "send-now" action returns an error).

### 4.2 Production deployment (`back.yml` + GitHub Actions)

`back.yml` contains placeholders that the deploy workflow substitutes from
repository secrets:

```yaml
brevo:
  enabled: ${BREVO_ENABLED}
  api_key: ${BREVO_API_KEY}
  sender_name: ${BREVO_SENDER_NAME}
  sender_email: ${BREVO_SENDER_EMAIL}
  api_url: ${BREVO_API_URL}
```

Add the corresponding entries in **GitHub → Settings → Secrets and variables →
Actions**. Following the convention already used by this repo (see
`.github/workflows/deploy.yml`), credentials go into **Secrets** and
configuration knobs go into **Variables**.

| Name                 | Kind          | Workflow reference         | Example value                          | Notes                       |
|----------------------|---------------|----------------------------|----------------------------------------|-----------------------------|
| `BREVO_ENABLED`      | **Variable**  | `${{ vars.BREVO_ENABLED }}`     | `true`                                 | `true` / `false` flag       |
| `BREVO_API_KEY`      | **Secret** 🔒 | `${{ secrets.BREVO_API_KEY }}`  | `xkeysib-...`                          | Sensitive — never commit    |
| `BREVO_SENDER_NAME`  | **Variable**  | `${{ vars.BREVO_SENDER_NAME }}` | `WattBrews Reports`                    | Display name in inbox       |
| `BREVO_SENDER_EMAIL` | **Variable**  | `${{ vars.BREVO_SENDER_EMAIL }}`| `reports@wattbrews.me`                 | Must be a verified sender   |
| `BREVO_API_URL`      | **Variable**  | `${{ vars.BREVO_API_URL }}`     | `https://api.brevo.com/v3/smtp/email`  | Override only if proxying   |

Only `BREVO_API_KEY` is a secret. The other four are non-sensitive deployment
configuration and belong under the **Variables** tab.

### 4.3 Wire them into `.github/workflows/deploy.yml`

Add a `sed` line per placeholder and the matching `env:` block entry. The keys
mirror the table above:

```yaml
      - name: Substitute env into back.yml
        run: |
          # ...existing sed lines...
          sed -i 's|${BREVO_ENABLED}|'"$BREVO_ENABLED"'|g' back.yml
          sed -i 's|${BREVO_API_KEY}|'"$BREVO_API_KEY"'|g' back.yml
          sed -i 's|${BREVO_SENDER_NAME}|'"$BREVO_SENDER_NAME"'|g' back.yml
          sed -i 's|${BREVO_SENDER_EMAIL}|'"$BREVO_SENDER_EMAIL"'|g' back.yml
          sed -i 's|${BREVO_API_URL}|'"$BREVO_API_URL"'|g' back.yml
        env:
          # ...existing env entries...
          BREVO_ENABLED:      ${{ vars.BREVO_ENABLED }}
          BREVO_API_KEY:      ${{ secrets.BREVO_API_KEY }}
          BREVO_SENDER_NAME:  ${{ vars.BREVO_SENDER_NAME }}
          BREVO_SENDER_EMAIL: ${{ vars.BREVO_SENDER_EMAIL }}
          BREVO_API_URL:      ${{ vars.BREVO_API_URL }}
```

After updating secrets/variables and the workflow, redeploy (push to `master`
triggers `.github/workflows/deploy.yml`).

---

## 5. Restart and verify

1. Restart the service:
   ```bash
   sudo systemctl restart evsys-back.service
   ```
2. Check the log for the initialization line:
   ```
   level=INFO msg="initializing brevo mail client" sender=reports@your-domain.com api_key=xkeys
   level=INFO msg="mail scheduler started"
   ```
   The `api_key` field is masked to its first five characters via `sl.Secret()`.

3. Verify the scheduler will fire next at the expected wake time (debug log):
   ```
   level=DEBUG msg="mail scheduler sleep" until=2026-04-27T06:00:00Z wait=10h32m...
   ```

---

## 6. Manage subscribers (admin UI)

Recipients are stored per-environment in the `mail_subscriptions` MongoDB
collection and managed via the admin frontend at **Manage → Mail reports**
(route: `/mail-subscriptions`). Each entry has:

| Field        | Meaning                                                     |
|--------------|-------------------------------------------------------------|
| `email`      | Recipient address (free-form, can be external)              |
| `period`     | `daily` / `weekly` / `monthly`                              |
| `user_group` | Same value the statistic page sends as `group` (e.g. `default`, `office`) |
| `enabled`    | Toggle without deleting                                     |

The `Send now` action (paper-plane icon) immediately dispatches a report for
that subscription using the current period's date range — useful for verifying
end-to-end delivery without waiting for the scheduler.

---

## 7. Schedule semantics

The scheduler runs in a single goroutine that wakes once per day at **06:00 UTC**:

| Period   | Fires when            | Date range covered                          |
|----------|-----------------------|---------------------------------------------|
| daily    | every day             | yesterday 00:00 UTC → today 00:00 UTC       |
| weekly   | Monday only           | previous Mon 00:00 → this Mon 00:00 (UTC)   |
| monthly  | 1st of the month only | previous month 1st 00:00 → this month 1st 00:00 (UTC) |

A given Monday-the-1st morning therefore fires all three periods. Delivery
errors are logged per-recipient and never abort the rest of the batch.

---

## 8. Report content

Each email is a single inline HTML table — no attachments. Columns:

- **Charger** — charge point identifier (rows sorted alphabetically)
- **Sessions** — number of completed transactions in the period
- **Energy (kWh)** — total energy delivered
- **Avg power (kW)** — average power across sessions

The data is identical to the frontend `/statistic` page filtered by `Clients`
(the subscription's `user_group`) and grouped by chargers. The same backend
endpoint feeds both: `GET /api/v1/report/charger?from&to&group=...`.

---

## 9. Troubleshooting

| Symptom                                  | Likely cause / fix                                                      |
|------------------------------------------|-------------------------------------------------------------------------|
| `brevo status 401: ...unauthorized...`   | API key wrong, revoked, or copied with extra whitespace.                |
| `brevo status 400: ...sender not allowed`| `sender_email` is not verified in Brevo.                                |
| Scheduler never starts                   | `brevo.enabled: false`, or the config block is missing entirely.        |
| Recipients receive nothing on Mondays    | The host clock is not UTC; the 06:00 tick refers to UTC, not local time.|
| `mail service not configured` from API   | Backend started with `brevo.enabled: false`; restart after enabling.    |
| `subscription not found` (404)           | Wrong `id`, or the document was deleted by another admin.               |

For deeper inspection of a failed send, raise the log level
(`env: dev` / `env: local`) and look for `send mail failed` entries — they
include the Brevo HTTP status code and response body.
