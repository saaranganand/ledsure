# ledsure

A double-entry, event-sourced ledger and payments service built to **prove
it stays correct under failure**: retries, crashes, and concurrent
requests. Money is never created, lost, or double-charged.

> **Status: M0 (Scaffold).** Only `/healthz` exists. Accounts, postings,
> balances, and idempotency land in later milestones. The full README this
> project is meant to have — architecture diagram, accounting model,
> invariants table, `curl` walkthrough, retried-request GIF — arrives once
> there is a ledger to demonstrate.

## Quickstart

```sh
docker compose up --build
curl http://localhost:8080/healthz
# {"status":"ok"}
```

## Project layout

| Path | Purpose |
|---|---|
| `cmd/ledsured/` | `main`: wiring only (config, logger, DB, router, server lifecycle) |
| `internal/ledger/` | Domain types, validation, posting rules — pure, no I/O |
| `internal/store/` | Postgres implementation (pgx) |
| `internal/api/` | HTTP handlers, idempotency middleware, error mapping |
| `internal/platform/` | `Clock`, `IDGenerator`, `Rand` seams + real implementations |
| `migrations/` | goose SQL migrations, embedded into the binary |

## Development

```sh
make up               # docker compose up --build
make test             # unit tests, -race
make test-integration # integration tests against real Postgres (testcontainers), -race
make lint             # golangci-lint
make down             # tear down and remove volumes
```

See the [Makefile](Makefile) for the rest of the targets and
[.env.example](.env.example) for configuration.
