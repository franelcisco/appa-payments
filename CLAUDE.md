# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Go service that handles Shopify checkout payments for the APPA storefront against Venezuelan payment rails (R4 bank API, BCV exchange rate, mobile payment "pago móvil", direct debit). It exposes a Gin HTTP server (default `:8080`) that the frontend calls to validate/process payments, look up Shopify orders, and request OTPs. Deployed as a distroless container (see `Dockerfile`) targeting Cloud Run.

Go module: `appa_payments` (Go 1.25). The module name has an underscore, so all internal imports use `appa_payments/...`, not `appa-payments/...`.

## Commands

```bash
# Run locally (reads .env via joho/godotenv autoload)
go run ./cmd

# Build the same binary Docker builds
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /tmp/server ./cmd

# Standard Go tooling — no Makefile in repo
go vet ./...
go build ./...
go test ./...             # domains + pkg/r4bank have tests; payouts store test needs PAYOUT_TEST_DSN (skipped without it)
gofmt -w .

# Container build
docker build -t appa-payments .
```

There is no lint config beyond `go vet` / `gofmt`. There is no test runner script — tests, when added, run with plain `go test`.

## Architecture

### Layered structure

```
cmd/main.go                wires everything: config → logger → db → external clients → services → handlers → routes
internal/
  config/                  env-var loader; Load() validates required vars and returns *Config
  routes/                  Gin route registration. Two route groups: StoreRoute, PaymentRoute
  handlers/                Gin handlers; thin — bind JSON, call service, map errors to HTTP
  services/                business logic (payments, store, webhook) + in-memory OTP cache
  domains/                 service interfaces and internal request DTOs (e.g. DirectDebitAccountRequest)
  models/                  HTTP request/response shapes used by handlers and services
pkg/                       reusable infrastructure clients (no business logic)
  db/                      gorm.Open + Postgres connection + DBRollback helper; schema.sql is reference, not migration
  db/models/               GORM models for r4_appa_* tables
  shopify/                 GraphQL admin API client + repository
  r4bank/                  R4 bank REST client + repository (HMAC-signed via helpers.GenerateAuthToken)
  bcv/                     BCV USD rate client; caches rate per day via SameDay check
  mailgun/                 transactional email (OTP, support alerts) with HTML templates
  drive/                   Google Drive uploads (payment receipts, etc.)
  logs/                    zap logger constructor
  helpers.go               package `helpers`, imported as `helpers "appa_payments/pkg"` — top-level pkg, not a subdir
```

`webhook.go` files in `internal/{domains,handlers,services,models}/` are present but untracked (visible in `git status`) and the worker pool in `handlers/webhook.go` is not yet wired into routes.

### Request flow

1. `cmd/main.go` builds external clients (`shopify`, `r4bank`, `bcv`, `drive`, `mailgun`) and the GORM `*gorm.DB`, then injects them into service constructors.
2. Services are returned as concrete `*paymentService` / store service types but consumed through `domains.PaymentService` / `domains.StoreService` interfaces by handlers.
3. Handlers bind JSON into `internal/models` structs and forward to services; errors become `500` with the raw error message, success returns JSON.
4. Payment services open GORM transactions and use `db.DBRollback(tx, &err)` deferred to commit on nil error or rollback on error/panic — the pattern relies on a named return `err` being assignable through the pointer.

### External integrations

- **Shopify Admin GraphQL** (`pkg/shopify`): order lookups, customer parent-ID metafield updates. Endpoint built from `SHOPIFY_STORE_NAME` + `SHOPIFY_API_VERSION` + `SHOPIFY_ADMIN_TOKEN`.
- **R4 bank** (`pkg/r4bank`): direct debit, mobile-payment validation, BCV rate. Requests are HMAC-SHA256 signed with `R4_SECRET` via `helpers.GenerateAuthToken`.
- **BCV** (`pkg/bcv`): wraps the R4 rate endpoint and caches today's rate in memory (`SameDay`). All bolívar amounts in services are derived as `usd * BCVTasa`.
- **Mailgun + Google Drive**: OTP and notification emails; receipt uploads. Drive uses OAuth via `GOOGLE_CREDENTIALS` + `GOOGLE_DRIVE_TOKEN` files.

### Payments domain — what to know before editing

- **OTP cache** (`internal/services/otp_cache.go`) is in-memory, mutex-protected, 2-minute TTL, single-use (`Validate` deletes on match). It is not durable — any restart drops codes. Replacing it requires changing the dependency in `NewPaymentService`.
- **Direct debit account** has two modes — first-charge and recurring — distinguished by Shopify order tags `direct_debit_account_firts` (sic) and `direct_debit_account_recurrent`. The `RECURRENT_DIRECT_DEBIT_APP_ID` env var identifies the app that owns the recurring charge metafield.
- **R4 codes reach the frontend; a refusal is not an error.** `internal/domains/r4.go` owns the typed `R4Code`, the code→Spanish description map, and `GetR4CodeDescription()` (`"Desconocido"` for a code with no text). The rule across every rail: **if R4 answered with a code at all — from the success body or from the error body — the endpoint responds `200` carrying that code plus its `message`.** `500` is reserved for the case where there is no code (transport failure, the r4-service's own `500`, or a failure before the call), because a call with no code may still be in flight at the bank and must not be rendered as a refusal.
- **The r4-service's own contract** (repo `boneappetit-r4-service`): `422` with `{error, code, operation_id}` for a coded refusal, `504` with a code on its timeout, `500` with only `{error}` otherwise. `pkg/r4bank/client.go` mirrors that split — `500` returns `(nil, err)`, any other non-2xx returns `(body, err)` — and `parseR4ErrorBody` in `pkg/r4bank/repository.go` turns that body into a normal response struct carrying `Code`/`ID`, for both `ValidateImmediateDebit` and `DirectDebitAccount`.
- **Domiciliación code mapping** lives in `directDebitAccountResponseCodes` (`internal/domains/direct_debit_account.go`): R4 returns `AM04`/`MD01`/`MD09`/`AC01`, we translate to `ERR01`–`ERR04`, the frontend renders Spanish copy. Add new codes there, not in handlers. A code with **no** mapping is passed through raw (with `message: "Desconocido"`) instead of failing the request, so the frontend must not assume `code` is always one of ours.
- **Reconciling an undetermined row.** `POST /payments/direct-debit/refresh-status` and `POST /payments/direct-debit-account/refresh-status` re-query R4 by `operation_id` and update the row, but only when its stored code is still undetermined (`domains.IsReconcilableCode`: `AC00`, `11`, `INT01`–`INT03`); a final code answers `409`. `INT01`–`INT03` are the r4-service's own codes for never having read a status. See `docs/refresh_status.md`.
- **No polling.** The r4-service resolves an operation to a final code before answering, so this service no longer polls `GetOperationByID` (`waitForOperationCompletion` and `awaitOperation` are gone). A break code (`AC00`/`11`) is returned to the frontend as-is; nothing finalizes that order later, same as in `bone_appetit_api`.
- **Customer DNI** comes from either the request (`dni` + `dniType`) or the Shopify customer's `ParentID` metafield (format `dniType-dni`). Use `helpers.GetCustomerDNI`, do not re-parse inline.
- **Amount comparison** uses a tolerance of `0.1 USD * BCVTasa` (≈10 ¢ in bolívars) when matching a recorded mobile payment against the order total. Greater amount → success with overpayment notice; lesser → failure path that emails support.

### Payouts — the one route that sends money OUT

`POST /payouts/vuelto` pushes a pago móvil (R4 `MBvuelto`) to a payee on behalf of a caller; APPA's admin uses it to pay claim reimbursements and clinic statements. Everything else here is public because the worst a stranger can do is ask about a payment. This route is different, and three rules hold it together — do not relax any of them:

- **Signed, whole-instruction.** Headers `X-Payout-Exp` + `X-Payout-Signature` = hex HMAC-SHA256 with `PAYOUT_SECRET` over `payoutId:bank:phone:dni:amount(%.2f):exp:concept` (`middleware.PayoutMessage`; APPA builds the same string in `_shared/vueltoPayout.ts`). `PAYOUT_SECRET` is optional in config on purpose: unset, the service boots and this route refuses everything. The headers are deliberately NOT in the CORS allow-list — no browser calls this.
- **Pay once per `payoutId`.** `r4_appa_payouts.payout_id` is UNIQUE and is reserved (`ON CONFLICT DO NOTHING`) *before* R4 is called. A repeated id never reaches R4; it gets the stored outcome back with `replayed: true`. If the table or index is missing the reservation errors and nothing is paid. `MBvuelto` has no idempotency key of its own — this is it.
- **Three outcomes, never two.** `confirmed` (with the R4 reference), `rejected` (R4 said no — safe to retry under a new id), `unknown` (no usable answer; the money MAY have moved, never retried). 200 always carries an outcome; every non-200 carries `sent: false`, a promise that R4 was not called, which the caller relies on. `r4bank.ClassifyVuelto` decides; when in doubt it says `unknown`.

The handler also validates shapes before verifying (uuid id, 4-digit bank, `04XXXXXXXXX` phone, `V12345678`-style document, whole céntimos, concept ≤ 60) — that is what keeps `:` out of the signed fields. A taken `payoutId` never gets `sent: false`, even when this request sent nothing: the id belongs to an earlier request that may have.

**"Not 00" is not "no".** The R4 service reports every R4 code other than `00` with one generic error and does not pass the code on, and R4's table includes in-process codes (`AC00`, time-outs) where the transfer may still go through. So today a non-00 answer is `unknown`, and a person reconciles it against the R4 statement. `rejected` only happens when the R4 service refused the request itself (400/401/403, before R4), or when it returns R4's `code` AND that code is listed in `VUELTO_REJECT_CODES` (comma-separated, empty by default). Turning that on needs two things outside this repo: the R4 service passing `code` through in its error body, and the list confirmed against R4's code table.

`ChangePaid` (refund Vueltos inside the payment flows) is untouched and still only returns an error.

### Database

Postgres via GORM, no migration framework. `pkg/db/schema.sql` is a hand-maintained reference for the two main tables (`r4_appa_debits_direct`, `r4_appa_debits_direct_account`); GORM models in `pkg/db/models/` are the source of truth at runtime and have drifted (columns like `is_recurring`, `draft_id`, `store_client_id` exist in the model but not the snapshot SQL). When the schema must change, update both the model and `schema.sql`, and apply the change directly against the database — there is no migration runner.

Transactions follow the pattern:

```go
tx := p.db.Begin()
var errDB error
defer db.DBRollback(tx, &errDB)
// ... assign errDB on failure paths and return; commit happens in the defer on success
```

### Configuration

All config is env-vars loaded by `internal/config.Load()`, which **fails fast** if any required var is missing (every field in the struct is required except `Port` and `Debug`). `.env` is autoloaded via `joho/godotenv/autoload` — production sets vars through the platform. `Debug=1` enables permissive CORS; otherwise `CORS_ALLOWED_ORIGINS` is parsed as comma-separated.

The server pins timezone to `America/Caracas` (`time.LoadLocation`) for all date-based logic.

### Logging

Use the injected `*zap.Logger` everywhere — no `fmt.Print*` for real logging (a few legacy debug prints remain in handlers/services). `logger.Sync()` is deferred in `main`.
