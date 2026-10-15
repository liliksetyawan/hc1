# Workshop Registration API

Backend for the "AI for Everyone" internal workshop registration case: a
30-seat quota, one registration per email, email-link verification with an
expiring hold on the seat, and no over-booking even when two employees
register for the last seat at the same instant.

Stack: Go 1.25, Gin (HTTP), GORM + Postgres (persistence), organized by the
DDD layering in `backend-skill-new/.ai/skill/STANDARDS.md`.

## Assumptions

- **Registering reserves a seat; it doesn't confirm one.** A new row is
  created with `status: "pending"` and immediately counts against the quota.
  The caller must hit `regis_url` (`GET
  /api/registrations/verify/<regis_code>`) within the TTL window to move it
  to `status: "success"`. If they don't, the row is treated as inactive the
  instant its TTL passes — the quota slot and the email are both free again
  — without any background job needed (see **Low-level design** below).
- **TTL default is 60 seconds**, configurable via
  `REGISTRATION_VERIFICATION_TTL_SECONDS` — short enough to demo the expiry
  behavior on request, long enough not to expire mid manual-testing.
- **The verify link is idempotent.** Hitting it again after success just
  re-reports `status: "success"` (HTTP 200) rather than erroring — this
  matters because it's a GET a browser might prefetch or a user might
  double-click.
- **Email uniqueness is scoped to *active* registrations**, not a DB-wide
  `UNIQUE` constraint: compared case-insensitively and after trimming
  whitespace, and only against rows that are `success` or still-unexpired
  `pending`. Once a pending row expires, that email is free to register
  again — matching "kuota dikembalikan" (quota returned) applying to the
  whole reservation, not just the numeric count.
- **`registrationId`** (`REG-0001` style) is derived from the row's
  monotonic DB primary key, not from "active count at creation time": since
  expiry can free a slot, active-count is not monotonic, and two different
  people could otherwise land on the same `REG-XXXX` over time.
- **Office-email domain is not restricted** — the brief says "email kantor"
  but names no company domain, so any syntactically valid email is accepted
  (`net/mail.ParseAddress`, rejecting display-name forms like `"Name
  <addr>"`).
- **Duplicate-email is checked before quota-full** when both could apply.
- **Validation status code is 422** (`VALIDATION_ERROR`); a malformed/non-JSON
  body is treated the same way.
- **No auth.** Out of scope for this exercise.
- `regis_code` is a 128-bit random hex token (`crypto/rand`); collision
  probability is negligible, so unlike the quota/duplicate checks it isn't
  re-verified under the advisory lock before insert — the DB's unique index
  on `regis_code` is the backstop.
- **Verification email is sent async, best-effort.** `regis_url` is always
  returned in the register response regardless of whether the email could be
  delivered — email is a convenience notification, not the source of truth.
  A failed send is logged, not surfaced to the caller, and does not roll
  back the registration: the quota slot is already correctly reserved, and
  if the person never gets the email, the reservation simply expires on its
  own TTL like any other unverified one.
- **No SMTP configured → no emails, not an error.** If `SMTP_USERNAME`/
  `SMTP_PASSWORD` aren't set, the server logs one warning at startup and
  simply never calls the mailer — registration and verification both still
  work purely via `regis_url` in the JSON response.

## Running locally

Requires Go 1.25+ (auto-installed by the toolchain directive in `go.mod`
if your local `go` is older) and a Postgres instance.

```bash
cp .env.example .env   # adjust DB_* / WORKSHOP_QUOTA_TOTAL / TTL if needed
set -a; source .env; set +a
docker compose up -d postgres
make run
```

`docker-compose.yml` maps Postgres to host port **5433** (not 5432) to avoid
clashing with any Postgres you might already have running locally — update
`DB_PORT` in `.env` if you change that mapping. Alternatively, skip
`docker-compose.yml` entirely and point `DB_*` at an existing local Postgres
— `internal/platform/config/config.go`'s defaults (`DB_PORT=5432`,
`DB_PASSWORD=root`) are set up for exactly that case; `database.Migrate`
creates `workshop_registrations` on it the same way either way, since the
app doesn't care which Postgres it's talking to. Note those config.go
defaults intentionally diverge from `docker-compose.yml`'s
(`POSTGRES_PASSWORD: postgres`, port 5433) — pick one path and keep `DB_*`
consistent with it, they aren't meant to be mixed.

The server applies both migrations on startup (idempotent — safe to restart
freely): `migrations/0001_create_workshop_registrations.sql` creates the
table, `migrations/0002_add_verification_to_workshop_registrations.sql` adds
the `status`/`regis_code`/`expires_at`/`verified_at` columns. Listens on
`:8080`. `GET /health` → `200 {"status":"ok"}`.

**To actually send verification emails** (optional — without this,
`regis_url` still works, just only via the API response), set in `.env`:

```bash
SMTP_HOST=smtp.gmail.com     # default, usually fine as-is
SMTP_PORT=587                # default
SMTP_USERNAME=you@gmail.com
SMTP_PASSWORD=xxxxxxxxxxxxxxxx   # Gmail App Password, NOT your login password —
                                  # generate one at myaccount.google.com/apppasswords
SMTP_FROM=you@gmail.com      # defaults to SMTP_USERNAME if unset
```

## Testing

```bash
make test        # go test ./... -cover
```

Business logic (duplicate/quota rules, the TTL expiry/reclaim behavior, the
concurrency guarantee, HTTP contract) is covered without needing a live
database — see **Test Results** below. The GORM repository itself
(`infrastructure/persistence`) is a thin mapping layer exercised indirectly
through this; it has been manually smoke-tested against a real
`docker compose up -d postgres` instance (register → duplicate → quota-full
→ concurrent-last-slot, 5 consecutive runs, all correct), but isn't covered
by an automated live-DB integration test in this repo.

## High-level architecture

```
                 ┌──────────────┐        ┌─────────────────┐
  HTTP request → │ transport/api│ ─────▶ │ mailer.SMTPMailer │ (async, best-effort)
                 └──────┬───────┘        └─────────────────┘
                        │  Gin handlers: bind/validate shape,
                        │  build regis_url, map errors → HTTP
                 ┌──────▼───────┐
                 │ application  │  RegisterHandler / VerifyHandler /
                 └──────┬───────┘  AvailabilityHandler: orchestrates rules
                        │
                 ┌──────▼───────┐
                 │   domain     │  Registration entity (IsActive/Verify),
                 └──────┬───────┘  Repository interface (no infra deps)
                        │
                 ┌──────▼───────┐
                 │infrastructure│  GORM implementation: transactions,
                 └──────────────┘  advisory lock, SQL
```

Dependencies point inward (transport/infrastructure depend on
application/domain; domain depends on nothing). `cmd/server/main.go` wires
the concrete GORM repository and SMTP mailer into the handlers at startup.
The mailer is a transport-layer concern, not application/domain: it's
triggered from `handleRegister` (not `RegisterHandler`) because building
the verification link needs the incoming HTTP request's host, which the
application layer deliberately has no access to.

## Low-level design

### The concurrency guarantee

The brief's critical scenario: two employees try to take the workshop's
last seat at nearly the same time — exactly one may succeed.

`RegisterHandler.Handle` (`internal/contexts/registration/application/register.go`)
runs the whole decision inside `Repository.WithTransaction`:

1. `tx.Lock(ctx)` — takes `pg_advisory_xact_lock(851274)`, scoped to the
   current DB transaction. If another registration transaction is
   mid-flight, this blocks until it commits or rolls back (Postgres
   releases the lock automatically either way).
2. `tx.ExistsActiveByEmail(ctx, email, now)` — duplicate check, evaluated first.
3. `tx.CountActive(ctx, now)` — current active (pending-unexpired + success)
   count; rejected if `>= quotaTotal`.
4. `tx.Create(ctx, reg)` — insert as `pending`, only reachable past both checks.

Because every registration attempt serializes on the same advisory lock,
steps 2–4 behave as one atomic read-then-write: there is no window where two
concurrent transactions both read "29/30" and both insert, pushing the count
to 31.

Verified by `TestRegisterHandler_ConcurrentLastSlot_OnlyOneSucceeds` and
`TestRegister_ConcurrentLastSlot_OnlyOneSucceeds` (application and HTTP
layers), plus a manual live run against real Postgres (5 consecutive races,
exactly one winner each time — see **Testing**). All run under `go test
-race`.

The in-memory `fakeRepository` used in tests reproduces this precisely: its
`Lock()` takes a `sync.Mutex` held for the whole transaction callback,
mirroring `pg_advisory_xact_lock`'s per-transaction scope — so the test
exercises the real locking *shape* of `RegisterHandler`, not a simplified
stand-in for the business rule.

### The TTL / quota-release mechanism

`Registration.IsActive(now)` (`internal/contexts/registration/domain/registration.go`)
is the single source of truth for "does this row occupy a slot":

```go
func (r *Registration) IsActive(now time.Time) bool {
    switch r.Status {
    case StatusSuccess: return true
    case StatusPending: return now.Before(r.ExpiresAt)
    default:            return false // expired
    }
}
```

`CountActive` and `ExistsActiveByEmail` both filter on this condition
(`status = 'success' OR (status = 'pending' AND expires_at > now())`) — so a
pending row stops counting **the instant** `expires_at` passes, with no
polling, cron, or background sweeper required. This is deliberately lazy:
nothing needs to "notice" the expiry for the slot to be free again; the next
`Count`/`Register`/`Verify` call simply computes the current truth.

Two things keep the stored `status` column itself honest, not just the
counts:

- `VerifyHandler`: `Registration.Verify` sets `Status = StatusExpired` and
  returns `ErrExpired` when called after `expires_at`, and the handler
  always persists that mutation via `Save`, even on the error path — a
  single-row flip, triggered by someone clicking a stale `regis_url`.
- `Repository.ExpirePending(ctx, now)`: a bulk
  `UPDATE workshop_registrations SET status = 'expired' WHERE status =
  'pending' AND expires_at <= now()`, called at the top of both
  `RegisterHandler.Handle` (inside the locked transaction, before the
  duplicate/quota checks) and `AvailabilityHandler.Handle` (before
  `CountActive`). This is the opportunistic sweep discussed when deciding
  against a cron job or a message queue for this: ordinary traffic on the
  two endpoints that already exist is enough to keep `status` fresh, with
  zero new infrastructure. If the app receives *no* traffic at all for a
  while, rows can still sit as literally `"pending"` past their TTL — but
  `CountActive`/`ExistsActiveByEmail` already exclude them correctly
  regardless, so nothing is ever double-counted even without a sweep
  having run. A cron/ticker would only buy accuracy for an out-of-band
  reader (e.g. an admin dashboard querying the DB directly) that never hits
  either endpoint — not needed here.

`GetAvailability` reads `CountActive` with no advisory lock (`ExpirePending`
and `CountActive` are two independent statements, not wrapped in a single
transaction): Postgres's read-committed isolation already guarantees it
never sees uncommitted rows from an in-flight registration, which is the
right amount of consistency for a dashboard counter, and a stray
unswept-but-already-excluded row changes nothing about correctness.

### Email delivery

`internal/platform/mailer.SMTPMailer` dials Gmail's SMTP server fresh per
send (`smtp.Dial` → `STARTTLS` on port 587 → `PLAIN` auth → `MAIL`/`RCPT`/
`DATA`), using `net/smtp` from the standard library — no SDK dependency.
`transport/api.Handler` depends on a small `Mailer` interface it defines
itself (`Send(ctx, to, subject, body) error`), not the concrete type, so
tests substitute a `fakeMailer` that records calls on a channel instead of
making real network calls.

`handleRegister` calls `sendVerificationEmailAsync` after
`RegisterHandler.Handle` succeeds but *before* writing the JSON response:

```go
regisURL := buildRegisURL(c, result.RegisCode)
h.sendVerificationEmailAsync(result.Name, result.Email, regisURL)   // go func() { ... }()
c.JSON(http.StatusCreated, registerResponse{ ...RegisURL: regisURL... })
```

The send itself runs in its own goroutine (`go func() { ... }()`), so a
slow or unreachable SMTP server never adds latency to the register response
— the HTTP response and the email send race independently. A failed send
(bad credentials, network issue, etc.) is only `log.Printf`'d; it cannot
fail the registration, because the quota slot is already correctly
committed by the time the goroutine runs. If `SMTP_USERNAME`/`SMTP_PASSWORD`
are unset, `main.go` passes a `nil` mailer and `sendVerificationEmailAsync`
no-ops — registration and `regis_url` work identically either way, the only
difference is whether an email actually goes out.

Verified by `TestRegister_SendsVerificationEmail` (asserts the mailer is
called with the registrant's email and a body containing the exact
`regis_url` from the response), `TestRegister_SucceedsEvenIfEmailSendFails`
(a `failingMailer` that always errors — registration still returns `201`),
`TestRegister_NilMailerDoesNotPanic`, and
`TestRegister_DoesNotSendEmailOnFailure` (a rejected registration, e.g.
`QUOTA_FULL`, never triggers a send).

## ERD

One table:

```
┌──────────────────────────────────────────────┐
│            workshop_registrations              │
├──────────────────────────────────────────────┤
│ id           BIGSERIAL     PK                  │
│ name         VARCHAR(255)  NOT NULL            │
│ email        VARCHAR(255)  NOT NULL            │
│ status       VARCHAR(20)   NOT NULL DEFAULT 'pending'
│                             CHECK IN ('pending','success','expired')
│ regis_code   VARCHAR(32)   NOT NULL, UNIQUE    │
│ created_at   TIMESTAMPTZ   NOT NULL DEFAULT now()
│ expires_at   TIMESTAMPTZ   NOT NULL            │
│ verified_at  TIMESTAMPTZ   NULL                │
└──────────────────────────────────────────────┘
```

No separate quota/counter table — `total` is a config value
(`WORKSHOP_QUOTA_TOTAL`, default 30) and `registered` is a `COUNT(*)` with
the active-row condition above, so there is nothing else that can drift out
of sync with it. `email` intentionally has no DB-level `UNIQUE` constraint
(see **Assumptions**) — duplicate prevention is enforced in
`RegisterHandler` under the advisory lock instead.

## Endpoints

### `POST /api/registrations`

Request:
```json
{ "name": "Karyawan A", "email": "a@corp.example" }
```

| Status | Code | When |
|---|---|---|
| 201 | – | reserved (pending); verification email sent async if SMTP is configured |
| 422 | `VALIDATION_ERROR` | missing name / invalid email format / unparsable body |
| 409 | `EMAIL_ALREADY_REGISTERED` | normalized email already has an active row |
| 409 | `QUOTA_FULL` | `registered >= total` |
| 500 | `INTERNAL_ERROR` | unexpected failure (e.g. DB unreachable) |

201 response:
```json
{
  "registrationId": "REG-0029",
  "name": "Karyawan A",
  "email": "a@corp.example",
  "status": "pending",
  "regis_url": "http://localhost:8080/api/registrations/verify/3f9a1c...e02b",
  "availability": { "total": 30, "registered": 29, "remaining": 1 }
}
```

422 response:
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Data yang dimasukkan belum valid.",
    "fields": [
      { "field": "name", "message": "Nama wajib diisi." },
      { "field": "email", "message": "Format email tidak valid." }
    ]
  }
}
```

409 response (duplicate or quota, `fields` omitted):
```json
{ "error": { "code": "QUOTA_FULL", "message": "Mohon maaf, kuota workshop sudah penuh." } }
```

### `GET /api/registrations/verify/:code`

Hitting `regis_url` from the register response.

| Status | Code | When |
|---|---|---|
| 200 | – | verified now, or already was (idempotent) |
| 400 | `VALIDATION_ERROR` | empty code |
| 404 | `REGISTRATION_NOT_FOUND` | no row with that `regis_code` |
| 410 | `REGISTRATION_EXPIRED` | TTL passed before verifying; quota already released |
| 500 | `INTERNAL_ERROR` | unexpected failure |

200 response:
```json
{ "registrationId": "REG-0029", "status": "success", "message": "Pendaftaran berhasil diverifikasi." }
```

410 response:
```json
{ "error": { "code": "REGISTRATION_EXPIRED", "message": "Kode registrasi sudah kedaluwarsa. Kuota telah dikembalikan." } }
```

### `GET /api/availability`

| Status | Code | When |
|---|---|---|
| 200 | – | ok |
| 503 | `INTERNAL_ERROR` | unexpected failure |

200 response:
```json
{ "total": 30, "registered": 12, "remaining": 18 }
```

## Test Results

```
$ go test ./internal/contexts/registration/... -race -cover
ok  	salt/hackethon/internal/contexts/registration/application	coverage: 94.9% of statements
ok  	salt/hackethon/internal/contexts/registration/domain	coverage: 100.0% of statements
ok  	salt/hackethon/internal/contexts/registration/transport/api	coverage: 96.2% of statements
```

<details>
<summary>All 45 test cases (verbose output)</summary>

```
domain:
  TestNewRegistration
  TestRegistration_IsActive (5 subtests: success/pending-before/at/after-expiry/expired)
  TestRegistration_Verify_Success
  TestRegistration_Verify_Expired
  TestRegistration_Verify_AlreadySuccess_Idempotent
  TestRegistration_Verify_AlreadyExpired
  TestNormalizeEmail

application:
  TestAvailabilityHandler
  TestAvailabilityHandler_RemainingNeverNegative
  TestAvailabilityHandler_SweepsExpiredRowStatus
  TestAvailabilityHandler_PropagatesRepositoryError (ExpirePending/CountActive)
  TestRegisterHandler_Success
  TestRegisterHandler_NormalizesEmailForStorageAndDuplicateCheck
  TestRegisterHandler_QuotaFull
  TestRegisterHandler_ValidationError
  TestRegisterHandler_ValidationError_EmptyEmail
  TestRegisterHandler_ExpiredPendingFreesSlot
  TestRegisterHandler_SweepsExpiredRowStatusOnRegister
  TestRegisterHandler_DuplicateEmailAllowedAfterExpiry
  TestRegisterHandler_PropagatesRepositoryErrors (Lock/ExpirePending/ExistsActiveByEmail/CountActive/Create)
  TestRegisterHandler_ConcurrentLastSlot_OnlyOneSucceeds
  TestVerifyHandler_Success
  TestVerifyHandler_IdempotentOnAlreadySuccess
  TestVerifyHandler_Expired
  TestVerifyHandler_NotFound
  TestVerifyHandler_EmptyCode
  TestVerifyHandler_PropagatesRepositoryErrors
  TestValidationError_Error

transport/api:
  TestRegister_Success
  TestRegister_DuplicateEmail
  TestRegister_QuotaFull
  TestRegister_ValidationError
  TestRegister_MalformedJSON
  TestRegister_InternalError
  TestRegister_ConcurrentLastSlot_OnlyOneSucceeds
  TestRegister_SendsVerificationEmail
  TestRegister_DoesNotSendEmailOnFailure
  TestRegister_NilMailerDoesNotPanic
  TestRegister_SucceedsEvenIfEmailSendFails
  TestAvailability_Success
  TestAvailability_InternalError
  TestVerify_Success
  TestVerify_NotFound
  TestVerify_Expired_ReleasesQuota
  TestVerify_InternalError
```

All pass under `-race`.

</details>

### Manual live-DB verification

Run against real Postgres — both via `docker compose up -d postgres && make
run`, and directly against a pre-existing local Postgres instance
(`DB_HOST=localhost`, default port/credentials, no docker-compose) — not
just the in-memory test fakes:

- Full contract walk (register → availability → duplicate → validation)
  matched the documented responses exactly, byte for byte.
- Filled the quota to 30/30 via 30 sequential registrations; the 31st was
  correctly rejected with `QUOTA_FULL`.
- Fired two truly concurrent `POST /api/registrations` at the last open
  slot, 5 times in a row (each round reset to 29/30 first): every round
  produced exactly one `201` and one `409 QUOTA_FULL`, final count never
  exceeded 30.
- Dumped the raw table mid-test and recomputed `IsActive` by hand per row to
  confirm it matched `CountActive`'s SQL exactly — the register/availability
  "mismatch" this surfaced during manual testing turned out to be the TTL
  legitimately expiring between calls spaced further apart than the TTL,
  not a calculation bug.
- Confirmed `ExpirePending`: registered a row, let its TTL lapse, then
  called `GET /api/availability` alone (no register in between) — the row's
  `status` in the DB flipped from `"pending"` to `"expired"` and the count
  dropped accordingly, proving the sweep runs correctly from the
  availability path too, not just from `RegisterHandler`/`VerifyHandler`.

## Demo script

1. `GET /api/availability` → `{"total":30,"registered":0,"remaining":30}`.
2. `POST /api/registrations` with a new name/email → `201`, `status:
   "pending"`, `remaining: 29`, and a `regis_url`. If `SMTP_USERNAME`/
   `SMTP_PASSWORD` are set, that email address also receives an actual
   confirmation email within a couple seconds containing the same
   `regis_url` — check the server log for `send verification email to ...`
   if it doesn't arrive, that's the delivery failure reason.
3. Repeat the same email → `409 EMAIL_ALREADY_REGISTERED`.
4. `GET` the `regis_url` from step 2 → `200 {"status":"success",...}`.
   `GET` it again → still `200` (idempotent).
5. Register again, but **don't** verify; wait past the TTL
   (`REGISTRATION_VERIFICATION_TTL_SECONDS`, default 60s — override it lower,
   e.g. `REGISTRATION_VERIFICATION_TTL_SECONDS=5`, to demo this quickly);
   `GET /api/availability` shows the slot freed (and the row's `status` in
   the DB is now actually `"expired"`, not just excluded from the count —
   the availability call itself swept it), and `GET` the stale `regis_url`
   → `410 REGISTRATION_EXPIRED`.
6. Repeat step 2 until `remaining: 0` → the next attempt with a fresh email
   → `409 QUOTA_FULL`.
7. `POST /api/registrations` with `{"name":"","email":"abc"}` → `422
   VALIDATION_ERROR` with both field errors.
