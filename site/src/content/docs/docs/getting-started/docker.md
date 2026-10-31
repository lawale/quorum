---
title: Docker
description: Run Quorum with Docker Compose or the published multi-arch images on GHCR, with or without the admin console and widgets.
sidebar:
  order: 3
---

## Docker Compose

The repository's Compose file starts PostgreSQL, runs migrations and launches Quorum with the admin console and widgets in one command:

```bash
docker compose up
```

The server container includes a built-in healthcheck that polls `/health` every 10 seconds. Dependent services, such as the seed and setup containers, wait for the server to be healthy before starting.

To start only the database, which is useful during local development:

```bash
docker compose up -d postgres
```

### Full demo with sample apps

```bash
docker compose --profile demo up
```

This starts Quorum, PostgreSQL, runs migrations, seeds sample data and launches the three sample consumer applications. See [Demo and sample apps](/docs/demo/) for details.

### Seed data only

```bash
docker compose --profile seed up
```

## Published images

Every release pushes multi-arch (amd64 and arm64) images to GitHub Container Registry:

| Image | Contents |
|-------|----------|
| `ghcr.io/lawale/quorum` | API only |
| `ghcr.io/lawale/quorum-console` | API and admin console |
| `ghcr.io/lawale/quorum-all` | API, admin console and widget bundle |
| `ghcr.io/lawale/quorum-demo-*` | The three sample apps |

Mount a config file and pass `-auto-migrate` to have the server apply its own migrations on start:

```bash
docker run -p 8080:8080 \
  -v /path/to/config.yaml:/etc/quorum/config.yaml:ro \
  ghcr.io/lawale/quorum-all:latest \
  -config /etc/quorum/config.yaml -auto-migrate -migrations /migrations
```

## Manual builds

```bash
# API only (no console, no widgets)
docker build -t quorum .

# With admin console
docker build -f Dockerfile.console -t quorum-console .

# With admin console and embeddable widgets
docker build -f Dockerfile.all -t quorum-all .

# Stamp the version reported by /health (release builds pass the git tag)
docker build --build-arg VERSION=v0.4.0 -t quorum .

# Run
docker run -p 8080:8080 -v /path/to/config.yaml:/etc/quorum/config.yaml quorum-all
```
