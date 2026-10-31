---
title: Quick start
description: Build Quorum from source, point it at PostgreSQL or SQL Server, run the migrations and start the approval API in a few minutes.
sidebar:
  order: 1
---

This gets a headless Quorum API running from source. For the admin console and embeddable widgets as well, see [Console and widgets locally](/docs/getting-started/full-stack/), or skip the build entirely with [Docker](/docs/getting-started/docker/).

## Prerequisites

- Go 1.26+
- PostgreSQL 14+ or SQL Server 2019+
- [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI

## 1. Get Quorum

```bash
git clone https://github.com/lawale/quorum.git
cd quorum
make build
```

## 2. Configure the database

Copy the example config and set your database credentials:

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your database details
```

## 3. Run migrations

Quorum keeps all of its tables in a dedicated `quorum` schema so it can share a database with your application. On PostgreSQL the schema must exist before the migrations run.

```bash
# PostgreSQL: create the quorum schema, then run migrations
psql "postgres://quorum:quorum@localhost:5432/quorum?sslmode=disable" -c "CREATE SCHEMA IF NOT EXISTS quorum;"
migrate -path migrations/postgres -database "postgres://quorum:quorum@localhost:5432/quorum?sslmode=disable&search_path=quorum" up

# SQL Server (schema is created automatically by migration 001)
migrate -path migrations/mssql -database "sqlserver://sa:Password@localhost:1433?database=quorum" up
```

## 4. Start the server

```bash
./bin/quorum -config config.yaml
```

The API is now live at `http://localhost:8080`. The default [authentication mode](/docs/operate/authentication/) is `trust`, which reads the caller's identity from headers such as `X-User-ID`, so you can exercise the API with curl straight away.

## Next steps

- Follow the [wire transfer walkthrough](/docs/getting-started/walkthrough/) to create a policy, a request and an approval.
- Read the [REST API](/docs/integrate/api/) overview, or use the [Go](/docs/clients/go/), [TypeScript](/docs/clients/typescript/) or [.NET](/docs/clients/dotnet/) client.
- Review the [configuration reference](/docs/operate/configuration/) before deploying.
