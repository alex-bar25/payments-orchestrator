# Payment Orchestrator

Go API and Next.js UI for a mock card lifecycle: create → authorize → capture → refund or cancel.

No real cards. No Stripe. Amounts are integer minor units. One `status` field, append-only `payment_events`, Postgres-backed `Idempotency-Key`.

![Payment Orchestrator UI](ui.png)

## State

```text
requires_payment → processing → authorized → captured → refunded
                         ↓            ↓
                      failed      cancelled
```

`failed`, `cancelled`, and `refunded` are terminal. Capture and refund are full-amount only.

The mock PSP treats amount `1` as an authorize decline, `2` as a capture decline (stays `authorized`), `3` as a refund decline (stays `captured`).

Same `Idempotency-Key` + same body returns the original row. Same key + different body is `409 IDEMPOTENCY_KEY_REUSE`.

Cancel voids `authorized` only (no PSP call). A recon worker compares `provider_ledger` to `payments.status` and writes `discrepancies` without changing payment status.

## Run

Postgres (Docker or Neon). Neon pooler URLs need simple protocol (`-pooler.` in `DATABASE_URL`).

```bash
export DATABASE_URL=postgres://payments:payments@127.0.0.1:5432/payments?sslmode=disable
export PORT=8080
```

```bash
make api-run      # :8080
make worker-run   # webhooks + recon
make web-dev      # :3000, proxies /api/v1 to the Go API
```

The UI defaults to `API_URL=http://127.0.0.1:8080`. Tests: `make api-test`.

## Host

Railway for the Go API. Vercel for the UI (`web/`). Neon for Postgres.

API variables: `DATABASE_URL`, `RUN_WORKER=1`. Vercel variable: `API_URL` = the Railway public URL.

Set a spending cap on Railway. Neon pooler URLs need `-pooler.` and `sslmode=require`. Do not put `DATABASE_URL` in git.

## API

| Method | Path                             |
| ------ | -------------------------------- |
| POST   | `/api/v1/payments`               |
| GET    | `/api/v1/payments/:id`           |
| POST   | `/api/v1/payments/:id/authorize` |
| POST   | `/api/v1/payments/:id/capture`   |
| POST   | `/api/v1/payments/:id/refund`    |
| POST   | `/api/v1/payments/:id/cancel`    |
| GET    | `/api/v1/payments/:id/events`    |
| POST   | `/api/v1/webhooks/mock`          |
| GET    | `/api/v1/discrepancies`          |

Errors look like `{ "error": { "code", "message" } }`. Mutations take `Idempotency-Key`.
