---
title: Configuration
description: Every Quorum configuration key with its default, and how to override any of them with QUORUM_ environment variables.
sidebar:
  order: 1
---

Quorum reads a YAML file passed with `-config`. See [`config.example.yaml`](https://github.com/lawale/quorum/blob/master/config.example.yaml) in the repository for a complete, commented example.

| Key | Default | Description |
|-----|---------|-------------|
| `server.host` | `0.0.0.0` | Listen address |
| `server.port` | `8080` | Listen port |
| `database.driver` | `postgres` | `postgres` or `mssql` |
| `auth.mode` | `trust` | `trust` or `verify` (`custom` planned) |
| `auth.verify.jwks_url` | | JWKS endpoint of your identity provider (required for `verify`; must be `https`, plain `http` only for localhost) |
| `auth.verify.issuer` | | Expected `iss` claim; empty disables the check |
| `auth.verify.audience` | | Expected `aud` claim; empty disables the check |
| `auth.verify.claims.user_id` | `sub` | Claim carrying the user ID |
| `auth.verify.claims.roles` | `roles` | Claim carrying roles; supports dotted paths and array or space/comma separated string values |
| `auth.verify.claims.permissions` | `permissions` | Claim carrying permissions, same shapes as roles |
| `auth.verify.claims.tenant_id` | `tenant_id` | Claim carrying the tenant ID |
| `auth.verify.claims.attributes` | | Map of attribute name to claim path, for [reviewer scoping](/docs/concepts/reviewer-scoping/) |
| `auth.verify.default_tenant_id` | | Tenant to use when the tenant claim is absent (single-tenant deployments) |
| `auth.verify.jwks_refresh` | `1h` | Background key refresh interval |
| `auth.verify.clock_skew` | `30s` | Leeway applied to `exp`, `nbf`, and `iat` |
| `auth.trust.attribute_headers` | | Map of attribute name to header name, for [reviewer scoping](/docs/concepts/reviewer-scoping/) |
| `webhook.max_retries` | `3` | Max webhook delivery attempts |
| `webhook.timeout` | `10s` | HTTP timeout per delivery attempt |
| `webhook.retry_interval` | `5s` | Base delay between retries (multiplied by attempt number) |
| `webhook.heartbeat` | `30s` | Safety-net polling interval for the outbox delivery worker |
| `expiry.check_interval` | `1m` | How often to check for expired requests |
| `metrics.enabled` | `false` | Enable Prometheus metrics |
| `metrics.path` | `/metrics` | Where the metrics are served |
| `console.enabled` | `false` | Enable admin console API routes |
| `console.jwt_secret` | (auto) | JWT signing secret for console sessions |
| `console.secure_cookies` | `false` | Set `true` in production (HTTPS) |
| `console.suggestions.roles_url` | `""` | External endpoint returning JSON `[]string` of available roles |
| `console.suggestions.permissions_url` | `""` | External endpoint returning JSON `[]string` of available permissions |
| `console.suggestions.auth_header` | `""` | Header name for upstream auth (e.g., `X-Api-Key`, `Authorization`) |
| `console.suggestions.auth_value` | `""` | Header value (e.g., `sk-abc123`, `Bearer token123`) |

## Environment variable overrides

Every config field can be overridden via environment variables using the `QUORUM_` prefix with `SCREAMING_SNAKE_CASE` paths. For example, `database.host` becomes `QUORUM_DATABASE_HOST`. Environment variables take the highest priority, overriding both YAML values and defaults. The config file path itself can be set via `QUORUM_CONFIG_PATH`.

## Command-line flags

| Flag | Purpose |
|------|---------|
| `-config <path>` | Config file to load |
| `-auto-migrate` | Apply pending migrations on start |
| `-migrations <dir>` | Where the migrations live when auto-migrating (the images ship them at `/migrations`) |
| `-health <url>` | Probe a running server's `/health` and exit non-zero if unhealthy. Used by the container healthcheck. |
