# Contributing to Quorum

Thanks for your interest in contributing to Quorum! This guide covers the development setup, project layout, and workflow for submitting changes.

## Prerequisites

- Go 1.25+
- Node.js 22+
- Docker (for PostgreSQL via testcontainers and docker compose)
- [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI
- [`golangci-lint`](https://golangci-lint.run/) v2.11+

## Getting Started

```bash
git clone https://github.com/lawale/quorum.git
cd quorum

# Start PostgreSQL
docker compose up -d postgres

# Run migrations
make migrate-up

# Build everything (console + widgets)
make build-all

# Copy and edit config
cp config.example.yaml config.yaml
```

## Project Structure

```
cmd/server/          — Entry point
internal/
  auth/              — Authentication providers and authorization hook
  config/            — Configuration loading (YAML + env var overrides)
  display/           — Display template resolution
  health/            — HealthChecker interface and component health aggregation
  metrics/           — Prometheus metrics
  model/             — Domain models
  server/            — HTTP handlers and routing
  service/           — Business logic and approval workflow
  sse/               — In-process pub/sub hub for SSE push notifications
  store/             — Storage interfaces
    postgres/        — PostgreSQL implementation
    mssql/           — SQL Server implementation
  webhook/           — Signal-driven outbox webhook dispatcher
console/
  console.go         — Embedded SPA handler (build tag: console)
  console_stub.go    — No-op when built without console tag
  frontend/          — Svelte 5 + TypeScript + Tailwind CSS
widgets/
  embed.go           — Embedded widget JS handler (build tag: embed)
  embed_stub.go      — No-op when built without embed tag
  frontend/          — Svelte 5 Web Components (custom elements)
migrations/
  postgres/          — PostgreSQL migration files
  mssql/             — SQL Server migration files
examples/
  banking/           — Go sample app (wire transfers)
  expenses/          — Node.js sample app (expense approval)
  access-portal/     — Python sample app (access requests)
scripts/
  seed.sh            — Sample data seeder
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `build` | Build the Go binary (no console, no widgets) |
| `build-console` | Build console frontend + Go binary with console |
| `build-embed` | Build widget frontend + Go binary with embeddable widgets |
| `build-all` | Build both frontends + Go binary with console and widgets |
| `run` | Build and start the server |
| `test` | Run unit tests with race detector |
| `test-integration` | Run store integration tests (requires Docker for testcontainers) |
| `test-all` | Run both unit and integration tests |
| `lint` | Run golangci-lint |
| `migrate-up` | Run PostgreSQL migrations |
| `migrate-down` | Roll back one PostgreSQL migration |
| `docker-up` | Start all services with docker compose |
| `docker-down` | Stop all docker compose services |
| `seed` | Run the seed script against a running server |
| `demo` | Start the full demo with sample apps via docker compose |
| `console-dev` | Start the console Svelte dev server |
| `embed-dev` | Start the widgets Svelte dev server |
| `clean` | Remove build artifacts |

## Frontend Development

To work on the console or widget frontends with hot reload, run the Go server and the Vite dev server side by side:

```bash
# Terminal 1 — API server (API-only binary is fine here)
make build && ./bin/quorum -config config.yaml

# Terminal 2 — Console dev server (proxies /api to localhost:8080)
make console-dev

# Terminal 3 — Widget dev server
make embed-dev
```

The console dev server runs at `http://localhost:5173/console/` with hot module replacement.

## Running Tests

```bash
# Unit tests
make test

# Integration tests (requires Docker — uses testcontainers)
make test-integration

# Both
make test-all
```

## Linting

```bash
make lint
```

This runs `golangci-lint` with the project's configuration.

## Submitting Changes

1. Fork the repository and create a branch from `master`.
2. Make your changes, ensuring tests pass (`make test`).
3. Run the linter (`make lint`) and fix any issues.
4. Write a clear commit message describing the change.
5. Open a pull request against `master`.
