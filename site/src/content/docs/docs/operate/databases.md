---
title: Databases
description: Quorum runs on PostgreSQL or Microsoft SQL Server and keeps everything in its own schema so it can share a database with your application.
sidebar:
  order: 3
---

Quorum supports PostgreSQL 14+ and Microsoft SQL Server 2019+. Set `database.driver` in your config:

```yaml
# PostgreSQL
database:
  driver: "postgres"
  host: "localhost"
  port: 5432
  params:
    sslmode: "disable"

# SQL Server
database:
  driver: "mssql"
  host: "localhost"
  port: 1433
  params:
    encrypt: "disable"
    TrustServerCertificate: "true"
```

## Schema isolation

Quorum creates all its tables in a dedicated `quorum` schema, including its migration history, so it can safely share a database with your application without table name conflicts.

On PostgreSQL the schema must exist before the migrations run:

```bash
psql "$DATABASE_URL" -c "CREATE SCHEMA IF NOT EXISTS quorum;"
```

The repository's Compose setup and `make migrate-up` do this for you. On SQL Server the first migration creates the schema itself.

## Migrations

Migrations are in `migrations/postgres/` and `migrations/mssql/` and use [`golang-migrate`](https://github.com/golang-migrate/migrate). Run them with the CLI as in the [quick start](/docs/getting-started/quickstart/), or let the server apply them on start with `-auto-migrate -migrations <dir>`, which is what the Docker images do.

## Durability guarantees

Status changes, audit entries and webhook outbox rows are written in one transaction. Whichever database you choose, a request can never be approved without its audit entry, and a webhook can never be lost between the state change and the delivery worker.
