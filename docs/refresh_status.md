# `refresh-status` — reconciling undetermined operations

Two endpoints that re-query R4 for a charge whose stored code never reached a
final state, and write back the answer.

```
POST /payments/direct-debit/refresh-status          → r4_appa_debits_direct
POST /payments/direct-debit-account/refresh-status  → r4_appa_debits_direct_account
```

Registered in `internal/routes/payments.go`, implemented at the end of
`internal/services/payments.go` (`RefreshDirectDebitStatus`,
`RefreshDirectDebitAccountStatus`, `resolveOperation`), handled in
`internal/handlers/payment.go`.

## Why these exist

A charge row is written with whatever code R4 gave, and some codes are not an
answer. `AC00` / `11` mean the bank has not decided yet; `INT01`–`INT03` are the
r4-service's own codes for *it* never having read one (see below). In all five
cases the money may or may not have moved, and nothing in this service resolves
the row later — no poller, no webhook, and nothing marks an order paid after the
fact.

These endpoints are the manual reconciliation path: an operator (or the panel
that reads the tables directly) sends the `operation_id` and gets the row
brought up to date.

## Request

```json
{ "operationId": "..." }
```

`operationId` is the only field and is required — the row is found by
`WHERE operation_id = ?`, indexed on both tables. Nothing else identifies the
operation, which is deliberate: a cart-path row has no `order_id` yet (see
[`docs/cart_payments.md`](cart_payments.md)) and would otherwise be unreachable.

## Reconcilable codes

`domains.IsReconcilableCode` (`internal/domains/r4.go`) — a stored code that is
still undetermined and therefore worth re-querying.

| Code | Origin | Meaning |
| --- | --- | --- |
| `AC00` | R4 | In progress; the bank has not decided. |
| `11` | R4 | Pending; same. |
| `INT01` | r4-service | Its budget ran out before it could read a single status. |
| `INT02` | r4-service | Its call to R4 failed in transport; whether it arrived is unknown. |
| `INT03` | r4-service | R4 answered with a body it could not parse. |

`INT0X` are the r4-service's `pkg/r4bank/errors.go` codes. They arrive here in
its `422`/`504` error body alongside `operation_id`, are parsed by
`parseR4ErrorBody` (`pkg/r4bank/repository.go`), and get stored like any other
code — which is why a row can hold one at all. **A debit may have been executed
under any of them.**

Anything else — `ACCP`, `AM04`, `MD01`, `MD09`, `AC01`, `FAI01`, or a code we
have no text for — is final here and refused with `409`.

## Flow

1. Look up the row by `operation_id`. Any failure → `404`.
2. `IsReconcilableCode(row.code)` false → `409`. **R4 is not called.**
3. `r4Repo.GetOperationByID`. Transport failure → `500`; the row is left alone
   and stays reconcilable.
4. R4 returns the same code the row already holds → `200` with a `null` body,
   no write.
5. Otherwise write `code` and `success = (code == "ACCP")` — a full-row `Save`
   — and answer `200` with the updated row.

## Responses

| Status | Body | When |
| --- | --- | --- |
| 200 | `{operationId, code, reference, success}` | The code moved; the row was updated. |
| 200 | `null` | R4 still reports the code the row already holds. Nothing was written. |
| 409 | `{"error": "la operación ya posee un código final"}` | The stored code is not reconcilable. |
| 404 | `{"error": "operación no encontrada"}` | No row with that `operation_id` — **and also any DB failure on the lookup**, which is folded into the same answer. |
| 400 | `{"error": "..."}` | Bind failure (missing `operationId`). |
| 500 | `{"error": "ocurrió un error al procesar la solicitud"}` | R4 unreachable, or the update failed. |

Two things the caller has to handle explicitly:

- **`null` is a success.** A `200` with no body means "asked, nothing changed" —
  the front must not treat it as an empty result or an error.
- **`reference` is the row's own**, echoed back as stored. The reference R4
  returns on the refresh is not written and not returned, so a row recorded
  after an `INT0X` failure keeps an empty `reference` even once it reconciles to
  `ACCP`.

The status split is the one the rest of the service follows: **500 means no code
was obtained**, never a refusal. See
[`docs/payments.md`](payments.md#response-contract).

## What it does not do

- **No Shopify side effects.** An operation reconciled to `ACCP` updates the row
  and stops. It does not mark the order paid, complete a draft, write a
  metafield, or tag anything — parity with `bone_appetit_api`, where nothing
  finalizes an order after the fact either. Closing the sale stays manual.
- **No sweep.** There is no batch or scheduled variant; each call reconciles one
  `operation_id`.
- **No auth or rate limit**, like every other endpoint in the group.

## Index

Neither table had an index on `operation_id` before this. Added to the GORM
models (`pkg/db/models/`) and to `pkg/db/schema.sql`; there is no migration
runner, so it must be applied by hand:

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_r4_appa_debits_direct_operation_id
    ON r4_appa_debits_direct(operation_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_r4_appa_debits_direct_account_operation_id
    ON r4_appa_debits_direct_account(operation_id);
```

Run each outside a transaction. `schema.sql` carries them without
`CONCURRENTLY`, matching the rest of that file.
