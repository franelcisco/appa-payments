# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Go service that handles Shopify checkout payments for the APPA storefront against Venezuelan payment rails (R4 bank API, BCV exchange rate, mobile payment "pago móvil", direct debit). It exposes a Gin HTTP server (default `:8080`) that the frontend calls to validate/process payments, look up Shopify orders, and request OTPs. Deployed as a distroless container (see `Dockerfile`) targeting Cloud Run.

Go module: `appa_payments` (Go 1.25). The module name has an underscore, so all internal imports use `appa_payments/...`, not `appa-payments/...`.

Canonical repository (latest): https://github.com/franelcisco/appa-payments

## Commands

```bash
# Run locally (reads .env via joho/godotenv autoload)
go run ./cmd

# Build the same binary Docker builds
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /tmp/server ./cmd

# Standard Go tooling — no Makefile in repo
go vet ./...
go build ./...
go test ./...             # currently no _test.go files in tree
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
- **R4 → frontend error mapping** lives in `directDebitAccountBankErrorCodes` (services/payments.go). The frontend pattern is: R4 returns `AM04`/`MD01`/`MD09`/`AC01`, we translate to `ERR01`–`ERR04`, the frontend renders Spanish copy. Add new codes here, not in handlers.
- **Customer DNI** comes from either the request (`dni` + `dniType`) or the Shopify customer's `ParentID` metafield (format `dni-dniType`). Use `helpers.GetCustomerDNI`, do not re-parse inline.
- **Amount comparison** uses a tolerance of `0.1 USD * BCVTasa` (≈10 ¢ in bolívars) when matching a recorded mobile payment against the order total. Greater amount → success with overpayment notice; lesser → failure path that emails support.

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
