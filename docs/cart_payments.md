# `/cart-payments/*` — reference

Cart-keyed counterpart to `/payments/*` (see [`docs/payments.md`](payments.md)),
for the checkout's `?cartId=` entry path. No Shopify Order or DraftOrder exists
yet on this path — Shopify only emails the buyer once one does, so the order is
minted at pay-time by the checkout's order-minting backend, **never by this
service**.

Registered in `internal/routes/cart_payments.go`, implemented in
`internal/services/cart_payments.go`.

## Architecture — how this differs from `/payments/*`

- **Authenticated by a signed quote, not a Shopify lookup.** Every endpoint in
  the group sits behind `CartQuoteRepository.Handler()`
  (`pkg/middleware/cart_quote.go`), applied at the router group in
  `routes/cart_payments.go` — including both `direct-debit-account/*otp*`
  endpoints. Four headers are required on every call.
- **The amount comes from the quote, in USD, already signed.** This service
  never re-derives it from a Shopify price; it only multiplies by the BCV rate
  (`amountVES = quote.Amount * bcv`). Whether anything was deducted before it
  was signed — a discount, a promo — is invisible here and stays that way; this
  service computes no discounts of its own on any path (see
  [`docs/payments.md`](payments.md#deliberately-not-implemented)).
- **This service performs zero Shopify order-mutating operations.** No tags, no
  discounts, no `draftOrderComplete`, no `markOrderAsPaid`, no metafield writes.
  It charges the bank rail, logs the attempt, and answers. Minting the order and
  every Shopify-side effect (customer metafield, order tags, discount) happen in
  the order-minting backend, which independently re-verifies the charge before
  acting on it — the same "server decides PAID, never the browser" rule
  `docs/payments.md` documents for the order/draft path, pushed one layer out:
  on cart, this service's response is an *input* to that decision, not the
  decision.
- **Reconciliation happens after the fact.** A charge here is logged against
  `cart_id` (plus a rail-specific `reference`) because no `order_id` exists yet.
  Once the order is minted, the minting backend calls
  `/cart-payments/attach-order` to backfill `order_id` / `order_name` (and, for
  domiciliación, `store_client_id`) onto that same row.

## The signed quote

| Header | Meaning |
| --- | --- |
| `X-Cart-Id` | Cart identity, in the form `gid://shopify/Cart/<id>?key=<key>`. |
| `X-Cart-Amount` | Total in **USD**, as a string. Must parse to a float `> 0`. |
| `X-Cart-Exp` | Unix seconds. Must be strictly in the future. |
| `X-Cart-Signature` | `helpers.GenerateAuthToken(CART_QUOTE_SECRET, "<cartId>:<amount>:<exp>")`, compared with `hmac.Equal`. |

Failures abort before any handler runs:

| Status | `code` | When |
| --- | --- | --- |
| 401 | `quote_unsigned` | A header is missing, or the signature is empty. |
| 401 | `quote_invalid` | Signature mismatch, or amount unparseable / `<= 0`. |
| 401 | `quote_expired` | `exp` is in the past. |
| 500 | `quote_misconfigured` | `CART_QUOTE_SECRET` is empty in this process. |

> **`CART_QUOTE_SECRET` is not validated at boot.** `config.Load()` fails fast on
> every other required var, but this one is read and never checked
> (`internal/config/config.go`). Deploy without it and the service starts
> healthy, serves `/payments/*` normally, and answers **every** cart endpoint
> with `500 {"code": "quote_misconfigured"}`.

> **CORS must name these headers.** They're custom, so a browser drops the
> request at preflight and the middleware never runs. `domains.CartQuoteHeaders`
> is appended to the allow-list in `cmd/main.go`; keep the two in sync when
> adding a header.

The `X-Cart-Id` format matters beyond identity: the pago-móvil path splits it
with `parseCartIDAndKey` to build the refund concept. An id without a `?key=`
part fails that split and the request returns a generic internal error — even
though the signature verified.

## Endpoints

All `POST`, all requiring the four headers above.

| Endpoint | Body | Rail |
| --- | --- | --- |
| `/cart-payments/generate-otp` | `bank`, `phone`, `dni`, `dniType` | Débito inmediato, step 1. R4 sends its own OTP to the buyer's phone — not Mailgun. |
| `/cart-payments/validate-direct-debit` | adds `name`, `otp`, `concept` | Débito inmediato, step 2. Moves the money. |
| `/cart-payments/validate-mobile-payment` | `bank`, `phone`, `reference`, `date` / `automatic`, `dni`, `dniType` | Pago Móvil. Matches an already-received R4 payment row — does not initiate a charge. |
| `/cart-payments/direct-debit-account` | `dni`, `account` (20 chars), `name` | Domiciliación, first-time affiliation. |
| `/cart-payments/direct-debit-account/request-otp` | `clientId` | Domiciliación, recurring — step 1. |
| `/cart-payments/direct-debit-account/otp` | `clientId`, `otp` | Domiciliación, recurring — step 2. Charges. |
| `/cart-payments/attach-order` | `reference`, `orderId`, `orderName`, `clientId?`, `paymentMethod` | All rails. Called by the minting backend after the order exists, never by the browser. |

There is no cart equivalent of `bcv-tasa` (use `GET /payments/bcv-tasa`) or of
`validate-mobile-payment-manual`.

## Débito inmediato

`generate-otp` converts the quote amount to VES and asks R4 to send the OTP.
`validate-direct-debit` charges and answers off the code R4 returns. The
r4-service resolves the operation to a final code before replying, so this path
**no longer polls** — `awaitOperation` and its ~45 s of in-request `GetOperationByID`
calls are gone. The order/draft path answers exactly the same way now
(`docs/payments.md`); the two are no longer opposites.

The row lands in `r4_appa_debits_direct` with `cart_id` set and
`order_type = "Cart"`. Response:

```json
{ "success": true, "code": "ACCP", "reference": "...", "message": "Transacción Exitosa" }
```

`success` is strictly `code == "ACCP"`. `message` is the description of that
code (`R4Code.GetR4CodeDescription()`), or `"Desconocido"` for a code we have no
text for — never an invented reason.

**No code at all** (transport failure, the r4-service's own 500) → HTTP 500 and
**no row is written**, only a log. Every case where R4 did answer with a code —
including a refusal or a break code — is a 200 carrying that code.

> **A cart débito inmediato left on a break code has no follow-up.** `AC00` /
> `"11"` come back with `success: false` and the row keeps that non-final code
> forever. If R4 later approves it, nothing here notices, nothing attaches an
> order, and the buyer has paid for an order that was never minted. Reconcile
> those rows out of band.

## Pago Móvil

Matches an unlinked row in `r4_appa_mobile_payments` — filter is
`order_id IS NULL AND cart_id IS NULL`, plus `issuing_bank`, `sender_phone`,
`reference LIKE '%<reference>'` (suffix), and today's date when
`automatic: true`, otherwise the supplied `date`. Retried 3 times with a 1 s
pause.

Amount is classified against `quote.Amount * bcv` with the shared
`0.1 USD * BCVTasa` tolerance (`domains.ClassifyCharge`):

| Verdict | What happens | `success` | `code` |
| --- | --- | --- | --- |
| Exact | Row stamped with `cart_id`. | `true` | *(empty)* |
| Overpaid | Row stamped, **excess refunded** via `ChangePaid`. | `true` | `over` |
| Underpaid | Row **deleted**, **full amount refunded**. | `false` | `under` |
| No match | Nothing. | `false` | `not_found` |

Refund attempts are recorded in `r4_appa_mobile_payment_reversals` with reason
`LESS` / `GREATER`; on the cart path the reversal's `order_name` column holds
the **cart id**, since no order name exists.

Note the code vocabulary here (`not_found` / `under` / `over`,
`internal/domains/mobile_payment.go`) is *not* the `OK` / `ERR0X` vocabulary the
domiciliación endpoints use.

## Domiciliación on cart — two separate flows

### `direct-debit-account` — first-time affiliation

The buyer types an account/DNI never used before; this charges the quote amount
(concept `"Prueba"`) to that account. On success the **minting backend** writes
it to the customer's `direct_debit_account` metafield — this service does not
touch Shopify.

Two persistence quirks in `registerDirectDebitAccountResult`, both deliberate,
both different from the order path:

- **The full account number is stored**, not the last 4 digits. `attach-order`
  reconciles this row from a *later*, separate request, and by then this is the
  only place the full number still exists.
- **`is_recurring` is written `true`** even for a first affiliation. The column
  does not distinguish affiliation from recurring charge on the cart path;
  `cart_id` plus `store_client_id` (still null until `attach-order`) is what you
  have.

### `request-otp` + `otp` — recurring

The buyer is already affiliated (metafield set by a prior order, draft, or cart
charge) and is paying *again*. There's no order or draft to read the affiliation
off, so the front sends `clientId` (the customer's Shopify GID) and this service
resolves it directly against the Admin API:

1. **`request-otp`** — `shopifyRepo.GetCustomerByID(clientId)`. Customer missing,
   or no `direct_debit_account` metafield → generic error, no distinct code. A
   6-digit code is generated, cached under `quote.CartID` (2-minute TTL,
   single-use, same cache type the order path keys by order id), and mailed via
   Mailgun **to the email Shopify has on file for that customer** — the request
   has no `email` field, so there is nothing for a caller to redirect.
2. **`otp`** — resolves the customer by `clientId` again (nothing from step 1 is
   trusted except the cached code), validates the OTP against the cart id, parses
   `account` / `dni` out of the metafield, and charges R4 with exactly those —
   never anything the request supplies beyond `clientId` and `otp`.

Result codes are the shared domiciliación set (`OK`, `AAF01`, `OTP01`,
`ERR01`–`ERR04`) — see the table in [`docs/payments.md`](payments.md#response-codes).
Two differences from the order path:

- `AAF01` here means "no customer, or no metafield" — reached before the OTP is
  even checked.
- **The affiliation self-heal does not run.** The order path deletes the
  metafield on `ERR02` / `ERR03`; the cart path only maps the code and returns
  it. Clearing the affiliation, if it should happen, is the minting backend's
  job.

## `attach-order`

Backfills the minted order onto the row this service already wrote. Matches
**`cart_id = <quote cart id>` AND `reference = <body reference>`** in the table
chosen by `paymentMethod`:

| `paymentMethod` | Table | Extra |
| --- | --- | --- |
| `direct_debit` | `r4_appa_debits_direct` | — |
| `mobile_payment` | `r4_appa_mobile_payments` | — |
| `direct_debit_account` | `r4_appa_debits_direct_account` | also writes `store_client_id` from `clientId` |

Anything else is rejected by binding (`oneof=`). **Zero rows matched is an
error**, not a no-op: `failed to attach order to payment` (HTTP 500), logged with
the cart id and reference.

The `cart_id` in the `WHERE` comes from the **verified quote**, not the body — so
the minting backend must present the same signed quote the browser used for the
charge. It cannot attach an order using only its own credentials.

## What this does NOT cover — read before extending

- **No proof of cart ownership on `clientId`.** Nothing checks that whoever holds
  this checkout tab is the customer behind `clientId`. A Shopify customer GID
  isn't secret. The design survives that — the OTP lands only in the real owner's
  inbox, so a caller who doesn't own the account can't *finish* a charge — but
  they can still (a) fire `request-otp` at someone else's `clientId`, unlimited,
  and (b) use the failure shape ("customer/metafield not found" vs "OTP invalid")
  as a light oracle for whether a `clientId` is affiliated at all.
- **No rate limiting anywhere.** Not by `clientId`, cart id, or caller IP.
  Nothing stops OTP brute-forcing inside the 2-minute window, or OTP-mail spam.
  Same as `/payments/*`.
- **The OTP cache is in-process memory** (`internal/services/otp_cache.go`), and
  `cartPaymentService` builds **its own instance**, separate from
  `paymentService`'s. It does not survive a restart between the two steps and
  does not work across more than one running instance — an instance that didn't
  serve `request-otp` has no record of the code `otp` is asked to validate.
- **Unmapped R4 codes come back as the bank's own code**, same as the rest of
  domiciliación (`docs/payments.md`): 200, `success: false`, `code` set to the
  raw R4 code, `"message": "Desconocido"`. The front must not assume `code` is
  always one of ours (`OK`, `ERR0X`).
- **No idempotency.** A retried `validate-direct-debit` charges again; a retried
  `direct-debit-account` charges again. There is no request key, and no check for
  an existing successful row on the same cart before charging.
