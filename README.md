# ledsure

A double-entry, event-sourced ledger and payments service built to prove
it stays correct under failure. Money is never created, lost, or double-charged.

## Quickstart

```sh
docker compose up --build
curl http://localhost:8080/healthz
# {"status":"ok"}
```

## Project layout

| Path | Purpose |
|---|---|
| `cmd/ledsured/` | `main`: config, logger, DB, router, server lifecycle |
| `internal/ledger/` | Domain types, validation, posting rules |
| `internal/store/` | Postgres |
| `internal/api/` | HTTP handlers, idempotency middleware, error mapping |
| `internal/platform/` | `Clock`, `IDGenerator`, `Rand` |
| `migrations/` | goose SQL migrations |

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
