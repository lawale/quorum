---
title: Console and widgets locally
description: Run the complete Quorum stack on your machine, with the admin console, the embeddable widgets and a local PostgreSQL in Docker.
sidebar:
  order: 2
---

The [quick start](/docs/getting-started/quickstart/) builds Quorum as a headless API server. To run the complete stack, with the admin console, the embeddable widgets and a local database, follow these steps.

## Prerequisites

- Go 1.26+
- Node.js 22+
- Docker (for PostgreSQL)
- [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI

## 1. Start PostgreSQL

```bash
docker compose up -d postgres
```

This starts a PostgreSQL 16 container with a `quorum` database and creates the `quorum` schema automatically. The database is available at `localhost:5432` with credentials `quorum:quorum`.

## 2. Run migrations

```bash
make migrate-up
```

## 3. Build with console and widgets

```bash
make build-all
```

This installs frontend dependencies, builds the console and widget bundles, and compiles the Go binary with both embedded. The console and the widgets are opt-in Go build tags; `make build-console` and `make build-embed` build one without the other.

## 4. Configure

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` and enable the console:

```yaml
console:
  enabled: true
```

## 5. Start the server

```bash
./bin/quorum -config config.yaml
```

Once running, the following endpoints are available:

| Endpoint | Description |
|----------|-------------|
| `http://localhost:8080/api/v1/` | REST API |
| `http://localhost:8080/console/` | Admin console |
| `http://localhost:8080/assets/embed.js` | Widget bundle |
| `http://localhost:8080/health` | Health check |

On first visit to the console you will be prompted to create an admin operator.
