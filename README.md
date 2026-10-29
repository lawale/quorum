# Quorum: The Open-Source Approval Engine for Modern Systems

[![CI](https://github.com/lawale/quorum/actions/workflows/ci.yml/badge.svg)](https://github.com/lawale/quorum/actions/workflows/ci.yml)
[![Security](https://github.com/lawale/quorum/actions/workflows/security.yml/badge.svg)](https://github.com/lawale/quorum/actions/workflows/security.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/lawale/quorum)](https://goreportcard.com/report/github.com/lawale/quorum)
[![npm](https://img.shields.io/npm/v/@oluwavader/quorum-embed)](https://www.npmjs.com/package/@oluwavader/quorum-embed)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lawale/quorum)](https://go.dev/)

**Every serious application eventually needs "someone else has to sign off on this". Quorum is that someone else, as a service.**

Wire transfers, production access, price changes, content going live: the [four-eyes principle](https://en.wikipedia.org/wiki/Two-man_rule) shows up everywhere, and every team rebuilds it from scratch. A status column, a couple of endpoints, an audit table that is never quite complete, a UI that only one app can use. Then the next app needs the same thing and it starts again.

Quorum pulls that logic out of your applications into one self-hosted service. Your app creates a request; Quorum matches it to a policy, walks it through the stages you defined, enforces who may act at each one, records everything, and tells you when it is decided. Rules live in data and change without a deploy. The approval UI is included as Web Components you drop into any front end. One engine, shared by every app that needs a second pair of eyes.

[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-316192?style=flat&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![SQL Server](https://img.shields.io/badge/SQL_Server-CC2927?style=flat&logo=microsoft-sql-server&logoColor=white)](https://www.microsoft.com/sql-server)
[![Prometheus](https://img.shields.io/badge/Prometheus-E6522C?style=flat&logo=prometheus&logoColor=white)](https://prometheus.io/)
[![Svelte](https://img.shields.io/badge/Svelte-FF3E00?style=flat&logo=svelte&logoColor=white)](https://svelte.dev/)
[![Lit](https://img.shields.io/badge/Lit-324FFF?style=flat&logo=lit&logoColor=white)](https://lit.dev/)

[Quick Start](#quick-start) • [Full Stack Local Setup](#running-the-full-stack-locally) • [Demo & Sample Apps](#demo--sample-apps) • [Key Features](#key-features) • [Why Quorum?](#why-quorum) • [Documentation](#documentation) • [Community & Support](#-community--support)

---

## Console Preview

| Requests Dashboard                                           | Policy Editor |
|--------------------------------------------------------------|---|
| ![Dashboard](docs/screenshots/console-dashboard.png) | ![Policy Editor](docs/screenshots/console-policy-editor.png) |

| Approval Detail | Audit Log |
|---|---|
| ![Approval Detail](docs/screenshots/console-approval-detail.png) | ![Audit Log](docs/screenshots/console-audit-log.png) |

---

## Key Features

- **Approval as a Shared Service** — Stop writing custom approval logic in every application. Quorum is one reusable service for all your multi-stage review workflows, keeping your core code focused on your domain. It complements your authorization layer rather than replacing it: your identity provider still says who a user is and what role they hold; Quorum decides whether this particular action has been reviewed by the right people.

- **Dynamic, Multi-Stage Policies** — Define complex approval rules with JSON. Create sequential stages (e.g., "Manager," then "Compliance"), each with its own required approvals and rejection rules. Change them at runtime without redeploying.

- **Full Workflow Lifecycle** — Create requests that require one or more approvals. Support for approve, reject, and cancel operations, all recorded in a complete audit trail.

- **Layered Permission Guards** — Control exactly who can approve at every level:
  - **Self-approval prevention** — The maker of a request can never approve their own request.
  - **Eligible reviewers** — Optionally restrict each request to a specific set of allowed reviewers.
  - **Role-based stage access** — Each approval stage can define which roles are allowed to act (e.g., only `manager` for Stage 1, only `compliance_officer` for Stage 2).
  - **Permission-based stage access** — Each approval stage can define which permissions are required (e.g., only users with `approve_transfers` permission for Stage 2). When both roles and permissions are configured, `authorization_mode` controls whether either suffices (`"any"`) or both are required (`"all"`).
  - **Bypass rules** — Declare which requests need no review at all, such as a root admin's own changes or transfers under a threshold. Matching requests are approved on creation, with the rule recorded in the audit trail. See [Bypass Rules](#bypass-rules).
  - **Reviewer scoping** — Each stage can require the checker's identity attribute (such as `branch_id` or `department`) to match the maker's, or a value in the payload, so a branch manager only acts on requests from their own branch. See [Reviewer Scoping](#reviewer-scoping).
  - **External authorization checks** — Delegate to an external HTTP endpoint for custom business logic (e.g., "only managers in the same department can approve").
  - **Duplicate vote prevention** — A checker can only act once per stage.
  - **Request fingerprinting** — Configurable identity fields prevent duplicate pending requests for the same underlying resource.

- **Universal Integration** — Plug Quorum into any stack using its simple REST API. Get notified of state changes via HMAC-signed webhooks backed by a **transactional outbox**, webhook entries are written atomically with the status update, so no event is ever lost, even if the process crashes mid-request. A signal-driven delivery worker ensures near-instant dispatch with a configurable heartbeat as a safety net.

- **Optional Admin Console** — Manage everything with a built-in web UI built with Svelte 5. Browse requests, define policies, inspect audit logs, and manage operators — all served directly from the Go binary. Opt-in via build tag for a smaller default binary.

- **Embeddable UI Widgets** — Drop pre-built Web Components into your own application to give end users approval functionality without building custom UI. Three widgets (`<quorum-approval-panel>`, `<quorum-request-list>`, `<quorum-stage-progress>`), built with Lit, use Shadow DOM for style isolation, work with any frontend framework, and are themeable via CSS custom properties and `::part()`. CORS is enabled out of the box for cross-origin embedding. Opt-in via build tag.

- **Real-Time Updates via SSE** — Widgets receive instant push notifications the moment a request's state changes, via Server-Sent Events. No polling required, the widget opens a long-lived connection and the server pushes events on approve, reject, cancel, stage advance, and expiry. Polling is kept as an automatic fallback if the SSE connection drops.

- **Production-Ready from Day One:**
  - **Multi-Database Support** — Choose between PostgreSQL and Microsoft SQL Server.
  - **Flexible Authentication** — `verify` mode validates JWT bearer tokens against your identity provider's JWKS endpoint (Keycloak, Auth0, Entra ID, Cognito, or anything OIDC-shaped), with configurable claim mapping and automatic key rotation. `trust` mode reads identity from headers set by your gateway or service mesh instead. See [Authentication Modes](#authentication-modes).
  - **Observability** — Export key metrics (request volume, approval rates, webhook delivery) to Prometheus for monitoring and alerting.
  - **Audit-Ready** — Every action is permanently logged with actor, timestamp, and details, giving you a complete history for compliance (SOC2, ISO 27001, etc.).

---

## Why Quorum?

| Rolling your own | With Quorum |
|---|---|
| Approval state lives in a column on your own table, with transitions scattered across handlers | Requests, stages, votes, and expiry are a first-class model with enforced transitions |
| Guards like "not the maker", "one vote per stage", "only managers" are re-implemented, and re-forgotten, per app | Layered guards are enforced centrally and configured per policy |
| Changing who approves what means a code change and a deploy | Policies are data; edit them at runtime in the console or API |
| The audit trail is whatever logging you remembered to add | Every action is recorded with actor, timestamp, and details, in a transactional outbox so nothing is lost |
| Each app builds its own approval screens | Drop in three themeable Web Components, or use the admin console |

- **For Developers:** Integrate a complete approval flow with a few API calls and a `<script>` tag. Focus on your product, not approval boilerplate.

- **For Security & Compliance Teams:** Enforce a consistent maker-checker principle across the organisation, and get one place to see every pending and decided critical action.

- **For Platform Engineers:** Offer approvals as a self-service capability on your internal platform, so other teams add governance to their tools without bespoke development.

---

## How It Works

```
┌──────────┐      ┌──────────┐      ┌──────────┐      ┌──────────┐
│   Your   │      │          │      │  Policy  │      │  Stage   │
│   App    │─────▶│  Quorum  │─────▶│  Match   │─────▶│  Check   │
│          │      │          │      │          │      │          │
└──────────┘      └──────────┘      └──────────┘      └─────┬────┘
  Creates            Receives         Finds policy          │
  Request            Request          for type              │
                                                            ▼
┌──────────┐      ┌──────────┐      ┌──────────┐      ┌──────────┐
│ Webhook, │      │ Request  │      │ Layered  │      │  Checker │
│ SSE, or  │◀─────│ Resolved │◀─────│ Guards   │◀─────│ Approves │
│  Poll    │      │          │      │          │      │          │
└──────────┘      └──────────┘      └──────────┘      └──────────┘
  Your app           Approved         Self-approval,        User
  acts on            Rejected         roles, permissions    submits
  result             Expired          are all verified      decision
```

---

## Quick Start

Get Quorum up and running in minutes.

### Prerequisites

- Go 1.26+
- PostgreSQL 14+ or SQL Server 2019+

### 1. Get Quorum

```bash
git clone https://github.com/lawale/quorum.git
cd quorum
make build
```

### 2. Configure Database

Copy the example config and set your database credentials:

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your database details
```

### 3. Run Migrations

```bash
# PostgreSQL — create the quorum schema, then run migrations
psql "postgres://quorum:quorum@localhost:5432/quorum?sslmode=disable" -c "CREATE SCHEMA IF NOT EXISTS quorum;"
migrate -path migrations/postgres -database "postgres://quorum:quorum@localhost:5432/quorum?sslmode=disable&search_path=quorum" up

# SQL Server (schema is created automatically by migration 001)
migrate -path migrations/mssql -database "sqlserver://sa:Password@localhost:1433?database=quorum" up
```

### 4. Start the Server

```bash
./bin/quorum -config config.yaml
```

The API is now live at `http://localhost:8080`. See the [API reference](#api-at-a-glance) below for your first request.

---

## Running the Full Stack Locally

The Quick Start above builds Quorum as a headless API server. To run the complete stack — admin console, embeddable widgets, and a local database — follow these steps.

### Prerequisites

- Go 1.26+
- Node.js 22+
- Docker (for PostgreSQL)
- [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI

### 1. Start PostgreSQL

```bash
docker compose up -d postgres
```

This starts a PostgreSQL 16 container with a `quorum` database and creates the `quorum` schema automatically. The database is available at `localhost:5432` with credentials `quorum:quorum`.

### 2. Run Migrations

```bash
make migrate-up
```

### 3. Build with Console and Widgets

```bash
make build-all
```

This installs frontend dependencies, builds the console and widget bundles, and compiles the Go binary with both embedded.

### 4. Configure

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` and enable the console:

```yaml
console:
  enabled: true
```

### 5. Start the Server

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

On first visit to the console you'll be prompted to create an admin operator.

---

## Use Case: Securing a Wire Transfer

This example shows how Quorum enforces a two-stage approval for a high-value wire transfer, preventing a single person from acting alone.

### 1. Define the Policy (One-Time Setup)

Create a policy that requires a manager's approval, followed by two compliance officers:

```bash
curl -X POST http://localhost:8080/api/v1/policies \
  -H "Content-Type: application/json" \
  -H "X-User-ID: admin" \
  -d '{
    "name": "High-Value Wire Transfer",
    "request_type": "wire_transfer",
    "stages": [
      {
        "index": 0,
        "name": "Manager Approval",
        "required_approvals": 1,
        "allowed_checker_roles": ["manager", "admin"],
        "rejection_policy": "any"
      },
      {
        "index": 1,
        "name": "Compliance Review",
        "required_approvals": 2,
        "allowed_checker_roles": ["compliance_officer", "admin"],
        "rejection_policy": "any"
      }
    ],
    "identity_fields": ["source_account_id"],
    "auto_expire_duration": "24h",
    "display_template": {
      "title": "Wire Transfer - {{amount | currency}}",
      "fields": [
        { "label": "From Account", "path": "source_account_id" },
        { "label": "Amount", "path": "amount", "format": "currency" },
        { "label": "Destination", "path": "destination" }
      ]
    }
  }'
```

The `display_template` tells Quorum how to render the payload for reviewers. See [Display Templates](#display-templates) for details.

### 2. Maker Creates a Request

A bank teller (`alice`) initiates a transfer. The request is now **pending**:

```bash
curl -X POST http://localhost:8080/api/v1/requests \
  -H "Content-Type: application/json" \
  -H "X-User-ID: alice" \
  -d '{
    "type": "wire_transfer",
    "payload": {
      "source_account_id": "ACC-001",
      "amount": 50000,
      "destination": "IBAN-12345"
    }
  }'
```

### 3. Checker Approves the Request

A manager (`bob`) approves. The request moves to the "Compliance Review" stage. Later, two compliance officers (`charlie` and `dave`) also approve, and the request is automatically marked as **approved**:

```bash
curl -X POST http://localhost:8080/api/v1/requests/{request_id}/approve \
  -H "X-User-ID: bob" \
  -H "X-User-Roles: manager" \
  -d '{"comment": "Funds available, looks good."}'
```

### 4. System Acts on the Result

Your application, listening via webhook, receives an `approved` event and executes the transfer; safe in the knowledge that all checks have been passed. The webhook is guaranteed to be delivered as it was written to a durable outbox in the same database transaction as the status change, so nothing can be lost.

---

## Permission Guards

Quorum enforces a layered permission model on every approval action. When a user attempts to approve or reject a request, each of these checks is evaluated in order:

| Guard | Scope | Description |
|-------|-------|-------------|
| **Self-approval prevention** | Built-in | The maker of a request cannot approve their own request. Always enforced. |
| **Eligible reviewers** | Per-request | An optional allowlist of user IDs set when creating the request. If provided, only those users can act. |
| **Allowed checker roles** | Per-stage | Each approval stage can restrict which roles are permitted. The checker must hold at least one matching role. |
| **Allowed permissions** | Per-stage | Each approval stage can restrict which permissions are required. The checker must hold at least one matching permission. |
| **Authorization mode** | Per-stage | When both `allowed_checker_roles` and `allowed_permissions` are set, controls whether either suffices (`"any"`) or both are required (`"all"`). |
| **Reviewer scope** | Per-stage | `scope` rules require the checker's identity attribute to equal the maker's attribute of the same name, or a value at a `payload_path`. See [Reviewer Scoping](#reviewer-scoping). |
| **Dynamic authorization URL** | Per-policy | An external HTTP endpoint called before each approval. The endpoint receives the full request context and returns `{"allowed": true/false}`. Use this for custom business logic your domain requires. |
| **Duplicate vote prevention** | Per-stage | A checker can only vote once per stage. They may vote on different stages of the same request. |
| **Request fingerprinting** | Per-policy | Configurable `identity_fields` extracted from the payload and hashed. Prevents duplicate pending requests for the same resource. |
| **Bypass rules** | Per-policy | `bypass_rules` describe requests that need no review. A matching request is approved on creation and audited as `bypassed`. See [Bypass Rules](#bypass-rules). |

```
Approve/Reject Request
        │
        ▼
┌───────────────────────┐
│  Self-approval check  │──▶ REJECT (maker cannot approve own request)
└───────┬───────────────┘
        │ pass
        ▼
┌───────────────────────┐
│  Eligible reviewers   │──▶ REJECT (checker not in allowlist)
│  (if configured)      │
└───────┬───────────────┘
        │ pass
        ▼
┌───────────────────────┐
│  Allowed checker      │──▶ REJECT (checker lacks required role)
│  roles (if set)       │
└───────┬───────────────┘
        │ pass
        ▼
┌───────────────────────┐
│  Allowed permissions  │──▶ REJECT (checker lacks required permission)
│  (if set)             │
└───────┬───────────────┘
        │ pass
        ▼
┌───────────────────────┐
│  Dynamic authorization│──▶ REJECT (endpoint returned allowed: false)
│  URL (if configured)  │
└───────┬───────────────┘
        │ pass
        ▼
   Vote recorded
```

### Dynamic Authorization Check

When a policy has a `dynamic_authorization_url`, Quorum POSTs the full request context (request ID, type, checker and maker IDs, their identity attributes, and payload) to your endpoint before allowing the approval. Your endpoint responds with `{"allowed": true}` or `{"allowed": false, "reason": "..."}`. This lets you implement arbitrary business rules — department separation, conflict-of-interest checks, rate limiting — without modifying Quorum.

---

## Documentation

All API endpoints are under `/api/v1` and require authentication (configurable via the `auth` section in config). The [wire transfer walkthrough](#use-case-securing-a-wire-transfer) above demonstrates the core request lifecycle.

### Configuration

See [`config.example.yaml`](config.example.yaml) for all available options.

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
| `auth.verify.default_tenant_id` | | Tenant to use when the tenant claim is absent (single-tenant deployments) |
| `auth.verify.jwks_refresh` | `1h` | Background key refresh interval |
| `auth.verify.clock_skew` | `30s` | Leeway applied to `exp`, `nbf`, and `iat` |
| `webhook.max_retries` | `3` | Max webhook delivery attempts |
| `webhook.timeout` | `10s` | HTTP timeout per delivery attempt |
| `webhook.retry_interval` | `5s` | Base delay between retries (multiplied by attempt number) |
| `webhook.heartbeat` | `30s` | Safety-net polling interval for the outbox delivery worker |
| `expiry.check_interval` | `1m` | How often to check for expired requests |
| `metrics.enabled` | `false` | Enable Prometheus metrics |
| `console.enabled` | `false` | Enable admin console API routes |
| `console.jwt_secret` | (auto) | JWT signing secret for console sessions |
| `console.secure_cookies` | `false` | Set `true` in production (HTTPS) |
| `console.suggestions.roles_url` | `""` | External endpoint returning JSON `[]string` of available roles |
| `console.suggestions.permissions_url` | `""` | External endpoint returning JSON `[]string` of available permissions |
| `console.suggestions.auth_header` | `""` | Header name for upstream auth (e.g., `X-Api-Key`, `Authorization`) |
| `console.suggestions.auth_value` | `""` | Header value (e.g., `sk-abc123`, `Bearer token123`) |

#### Environment Variable Overrides

Every config field can be overridden via environment variables using the `QUORUM_` prefix with `SCREAMING_SNAKE_CASE` paths. For example, `database.host` becomes `QUORUM_DATABASE_HOST`. Environment variables take the highest priority, overriding both YAML values and defaults. The config file path itself can be set via `QUORUM_CONFIG_PATH`.

### Authentication Modes

Authentication is selected via `auth.mode`. Both implemented modes produce the same identity (user ID, roles, permissions, tenant), so policies and the rest of the system do not care which one is in use.

#### `verify` — JWT validation against your identity provider

Quorum validates the `Authorization: Bearer <jwt>` header on every request against the signing keys published at your provider's JWKS URL. This is the mode to use when browsers or third-party services call Quorum directly, including the embeddable widgets.

```yaml
auth:
  mode: verify
  verify:
    jwks_url: https://login.example.com/realms/acme/protocol/openid-connect/certs
    issuer: https://login.example.com/realms/acme
    audience: quorum
    claims:
      user_id: sub
      roles: realm_access.roles   # Keycloak nests roles; dotted paths are supported
      tenant_id: tenant_id        # custom claim added by a mapper
```

What is enforced:

- **Signature** against a key from the JWKS, matched by `kid`. Only asymmetric algorithms (RS*, PS*, ES*) are accepted; HMAC and `none` are rejected regardless of the token header.
- **Expiry** (`exp`, required), `nbf`, and `iat`, with `clock_skew` leeway.
- **Issuer** and **audience**, when configured.
- **Identity claims**: the user ID claim must be present. The tenant claim must be present unless `default_tenant_id` is set.

Key rotation is automatic: keys are refreshed in the background every `jwks_refresh`, and a token signed with an unknown `kid` triggers an immediate refresh (rate-limited to once a minute, so a flood of bad tokens cannot be used to hammer your provider). If a refresh fails the previously cached keys stay in use, and if the provider is unreachable at startup Quorum still starts and fetches keys on the first request.

Roles and permissions accept the shapes providers actually emit: a JSON array of strings, or a single space- or comma-separated string such as an OAuth `scope` claim.

Auth0 example, where roles usually arrive under a namespaced claim and permissions in `permissions`:

```yaml
auth:
  mode: verify
  verify:
    jwks_url: https://acme.us.auth0.com/.well-known/jwks.json
    issuer: https://acme.us.auth0.com/
    audience: https://quorum.acme.internal
    claims:
      roles: https://acme.com/roles
      permissions: permissions
      tenant_id: https://acme.com/tenant
```

#### `trust` — identity from gateway headers

Reads user identity from request headers (`X-User-ID`, `X-User-Roles`, `X-User-Permissions`, `X-Tenant-ID`; names are configurable). Suitable when Quorum sits behind a gateway, service mesh, or backend-for-frontend that has already authenticated the user and sets these headers. Quorum must not be reachable directly by end users in this mode, since anyone who can set the headers can impersonate anyone.

#### `custom` *(planned)*

Delegate authentication to an external HTTP endpoint, for identity systems that do not issue JWTs.

Note that stage-level [external authorization checks](#key-features) are a separate, implemented feature: once a user is authenticated, a policy can still call out to your endpoint to decide whether that user may act on a given stage.

### Database Support

Quorum supports PostgreSQL and Microsoft SQL Server. Set `database.driver` in your config:

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

**Schema isolation:** Quorum creates all its tables in a dedicated `quorum` schema, so it can safely share a database with your application without table name conflicts. The schema must be created before running migrations (see [Run Migrations](#3-run-migrations) or `make migrate-up`).

Migrations are in `migrations/postgres/` and `migrations/mssql/` respectively.

### Admin Console

Quorum includes an optional embedded admin console, a web UI for managing policies, webhooks, viewing requests, and browsing audit logs. It is **opt-in** via a Go build tag. The standard binary has no frontend assets and no `/console` route.

**Build with the console:**

```bash
make build-console
```

**Enable in config:**

```yaml
console:
  enabled: true
  jwt_secret: ""  # optional; auto-generated if empty (sessions lost on restart)
```

Visit `http://localhost:8080/console/` after starting the server. On first run, you'll be prompted to create an initial admin operator. Subsequent operators can be added through the UI.

**Role & permission suggestions:** Optionally configure `roles_url` and `permissions_url` to point at your identity provider's API. The console proxies these URLs server-side and populates dropdown suggestions in the policy editor, while still supporting free-text entry. The endpoints must return a JSON array of strings (e.g., `["admin", "manager", "viewer"]`).

```yaml
console:
  enabled: true
  suggestions:
    roles_url: "https://your-idp.example.com/api/roles"
    permissions_url: "https://your-idp.example.com/api/permissions"
    auth_header: "X-Api-Key"
    auth_value: "your-idp-api-key"
```

**Console features:** Dashboard overview, policy CRUD with multi-stage editor (role/permission inputs with external suggestions, authorization mode dropdown, client-side validation), webhook management with delivery rate color coding, request browser with status/type filters and copy-to-clipboard IDs, request detail with tabbed payload and audit trail, audit log with policy context, delivery browser with partial event matching, and operator management.

### Embeddable Widgets

Quorum provides three Web Components that you can embed directly in your application's UI. They connect to the Quorum API and render approval workflows — no custom UI code needed.

**Build with widgets:**

```bash
make build-embed
```

**Build with both console and widgets:**

```bash
make build-all
```

The bundle is served at `/assets/embed.js` with CORS headers, so you can load it from any origin.

**Option 1: Script tag** — load from your Quorum server:

```html
<script src="https://your-quorum-host/assets/embed.js"></script>
```

**Option 2: npm** — bundle into your own build:

```bash
npm install @oluwavader/quorum-embed
```

```typescript
// Registers all three custom elements
import '@oluwavader/quorum-embed';

// Optionally use the API client directly
import { createClient } from '@oluwavader/quorum-embed';

const client = createClient({
  apiUrl: 'https://your-quorum-host',
  token: session.accessToken, // sent as "Authorization: Bearer ..."; server in verify mode
});
const request = await client.getRequest('uuid-here');
```

**Configure once, then embed:**

```js
// With the script tag the API is on window.QuorumEmbed
QuorumEmbed.configure({
  apiUrl: 'https://your-quorum-host',
  token: () => session.accessToken, // static value or getter, called on every request
});
```

```html
<!-- Show approval details with approve/reject actions -->
<quorum-approval-panel request-id="uuid-here"></quorum-approval-panel>

<!-- List requests with filters and pagination -->
<quorum-request-list status="pending" page-size="10"></quorum-request-list>

<!-- Visualize approval stage progress (with real-time SSE updates) -->
<quorum-stage-progress request-id="uuid-here" poll-interval="30000"></quorum-stage-progress>
```

Any attribute (`api-url`, `token`, `tenant-id`, `auth-headers`) set on an element overrides the global value for that element.

**Authentication:** With the server in `verify` mode, pass the user's access token via `token` (or a getter in `configure()`); it is sent as `Authorization: Bearer <token>` and the tenant comes from the token's claims. With the server in `trust` mode, pass `auth-headers` (a JSON string) such as `{"X-User-ID": "alice", "X-User-Roles": "manager"}` instead, and make sure the page is served by a backend that already knows the user, as the sample apps do, with Quorum not exposed directly. Approve and Reject buttons only render when the server reports the viewer may act on the request.

**Theming:** Set CSS custom properties such as `--quorum-accent`, `--quorum-bg`, or `--quorum-font` on any ancestor to restyle every widget, and use `::part()` selectors for structural overrides. The full token and part list is in the [package README](widgets/frontend/README.md#theming).

**Real-time updates:** Both the approval panel and stage progress widgets connect via SSE by default for instant push notifications. Set `sse="false"` to disable and use polling only. The `poll-interval` attribute (default 30000ms) controls the fallback polling rate. When both widgets are on the same page, the stage progress widget also reacts to custom events dispatched by the approval panel for immediate cross-widget updates.

**Error handling:** Set `suppress-errors` on the widget to hide inline error messages. Listen for `quorum:error` events to handle errors externally (e.g., toast notifications). The event detail includes `{ action, message, status }` where `action` is `"load"`, `"approve"`, or `"reject"`.

**Events:** Widgets dispatch custom events you can listen for: `quorum:approved`, `quorum:rejected`, `quorum:select`, and `quorum:error`.

### Bypass Rules

Not everything needs a second pair of eyes. A root administrator correcting a branch address, or a transfer under a small threshold, usually goes straight through in a hand-rolled maker-checker, and that exemption lives in code where nobody can see it. Quorum makes it policy.

```json
"bypass_rules": [
  {
    "name": "root admin",
    "maker_roles": ["root_admin"]
  },
  {
    "name": "small internal transfer",
    "maker_attributes": { "branch_id": ["HQ"] },
    "payload": [
      { "path": "amount",   "op": "lt", "value": 1000 },
      { "path": "currency", "op": "in", "value": ["NGN", "GHS"] }
    ]
  }
]
```

When a request is created, Quorum tries the rules in order. A rule matches when **every** condition it specifies holds:

- **`maker_roles`** — the maker holds at least one of these roles.
- **`maker_permissions`** — the maker holds at least one of these permissions.
- **`maker_attributes`** — for each attribute listed, the maker's value is one of the allowed values. Attributes come from the same configuration as [Reviewer Scoping](#reviewer-scoping).
- **`payload`** — every condition is true for the request payload. `op` is one of `eq`, `ne`, `lt`, `lte`, `gt`, `gte`, `in`. Ordering operators compare numbers only; `in` takes an array. A missing path never satisfies a condition, including `ne`.

A rule must specify at least one condition; an empty rule is rejected, since it would bypass everything.

**Velocity limits.** A threshold on its own invites structuring: anyone who learns that transfers under 1,000 skip review will send 900 five times. Give the rule `limits` and it stops matching once recent activity is too high:

```json
{
  "name": "small internal transfer",
  "payload": [ { "path": "amount", "op": "lt", "value": 1000 } ],
  "limits": [
    { "window": "24h", "max_count": 3 },
    { "window": "24h", "max_sum": { "path": "amount", "value": 2500 } },
    { "window": "7d",  "max_sum": { "path": "amount", "value": 5000 }, "group_by": ["payload:destination_account"] }
  ]
}
```

- **`window`** — how far back to look. Go durations such as `24h` or `90m`, plus a `d` suffix for days.
- **`max_count`** — the most requests allowed in the window, counting the one being created.
- **`max_sum`** — the most a numeric payload field may total across the window, again including the new request. Numeric strings are accepted.
- **`group_by`** — what makes two requests "the same activity". Defaults to `["maker"]`. Add `payload:<path>` entries to group by a payload value instead of, or as well as, the maker. The third limit above catches five different people each sending 900 to one destination account.

Every request of the policy's type in the window counts, whatever its status and whether or not it was bypassed. When any limit would be exceeded the rule does not match and Quorum tries the next rule; if none matches, the request goes through the normal stages. Bypass fails safe into review, never into rejection, and a database error during the check is treated the same way.

When a limit is what sent a request to review, Quorum writes `metadata.bypass_limit_exceeded` with the rule, window, measure, observed value and limit, and records a `bypass_limit_exceeded` audit entry. The console and the approval panel widget show this as a notice above the request, so the reviewer knows they are looking at the fourth transfer today rather than just another 900.

Limits make a bypass threshold safe to offer. They are not transaction monitoring: patterns across request types or tenants, or over months, belong in the AML tooling behind your application. Quorum's job is to make sure that when the pattern is visible at approval time, a human sees it.

On the first match the request is created and approved in one step. No approval rows are written. The audit log records `created` followed by `bypassed` with the rule name, the usual `approved` webhooks fire, and SSE subscribers are notified. The response to the create call already carries `"status": "approved"`, so your application can act immediately. Requests that match no rule proceed through the stages as normal.

Bypass applies to the whole policy, not to a stage. If what you want is "head office may approve any branch's request", that is a [scope](#reviewer-scoping) rule on the stage, not a bypass.

### Reviewer Scoping

Roles say *what kind* of person may approve. Scope says *which* of them: the manager of the same branch, a lead in the same department, the owner of the affected cost centre. Without it, every manager in the organisation can act on every branch's requests.

**1. Tell Quorum which identity attributes to read.** In `trust` mode they come from headers; in `verify` mode from JWT claims. The names you choose here are the names policies refer to.

```yaml
auth:
  trust:
    attribute_headers:
      branch_id: "X-User-Branch"
  verify:
    claims:
      attributes:
        branch_id: "branch_id"           # or a dotted path such as org.branch.id
```

Only scalar values are read. Quorum captures the maker's attributes on every request it creates and stores them as `maker_attributes`.

**2. Add `scope` rules to a stage.**

```json
{
  "index": 0,
  "name": "Branch Manager",
  "required_approvals": 1,
  "allowed_checker_roles": ["branch_manager"],
  "rejection_policy": "any",
  "scope": [
    { "attribute": "branch_id" }
  ]
}
```

This stage now accepts a checker only if their `branch_id` equals the maker's `branch_id`.

A rule names an **attribute** and one or more **sources** of acceptable values. The checker passes the rule when their attribute equals any value the sources produce. Every rule on a stage must pass.

- **`attribute`** — the identity attribute name, as configured above. Required.
- **`match_maker`** — accept the maker's attribute of the same name, captured when the request was created. This is the default when no other source is given.
- **`payload_path`** — accept the value at a dot-notation path in the request payload, for example `"destination_branch_id"`. Use this when a stage belongs to a branch other than the maker's.
- **`values`** — accept any of a fixed list, for example `["HQ"]`.
- **`allow_unscoped_checkers`** — when `true`, a checker who has no value for the attribute at all may act on any request. A checker who *does* have the attribute is still held to it. Defaults to `false`.

Because scope is per stage, a request can travel between branches:

```json
"stages": [
  { "index": 0, "name": "Originating branch", "required_approvals": 1, "rejection_policy": "any",
    "scope": [{ "attribute": "branch_id" }] },
  { "index": 1, "name": "Receiving branch or head office", "required_approvals": 1, "rejection_policy": "any",
    "scope": [{ "attribute": "branch_id", "payload_path": "destination_branch_id", "values": ["HQ"] }] }
]
```

Stage 0 is reviewed in the maker's branch. Stage 1 is reviewed by the branch named in the payload, or by head office.

Attribute names in a policy must match the names configured in step 1. Saving a policy whose scope rules reference an attribute the active auth mode does not supply still succeeds, but the response carries a `warnings` array (and the console shows it) explaining that the stage can never be approved as configured.

Rules fail closed: if no source yields a value, because the maker had no such attribute and the payload path is missing, the checker is out of scope. Out-of-scope decisions return `403`, and `viewer_can_act` on a request reflects the scope check so widgets hide the buttons.

Both sets of attributes are also passed to the [dynamic authorization URL](#permission-guards) as `maker_attributes` and `checker_attributes`, so an external check can build on them.

### Display Templates

By default, reviewers see the raw JSON payload — which often contains system IDs and machine-oriented data. Display templates let you define how a payload should be presented to human reviewers.

**How it works:**

1. Define a `display_template` on your policy with field labels, paths into the payload, and optional formatters.
2. When a request is created, Quorum resolves the template against the payload and stores the result in `metadata.display`.
3. The widgets and console render the resolved fields as a clean label-value view instead of raw JSON.

Templates are resolved once at creation time, so editing a policy template only affects future requests.

**Template format:**

```json
{
  "title": "Wire Transfer - {{amount | currency}}",
  "fields": [
    { "label": "From Account", "path": "source_account_id" },
    { "label": "Amount", "path": "amount", "format": "currency" },
    { "label": "Destination", "path": "destination" }
  ],
  "items": {
    "path": "profiles",
    "label_path": "name",
    "fields": [
      { "label": "Email", "path": "email" },
      { "label": "Role", "path": "role" }
    ]
  }
}
```

- **`title`** — interpolated string using `{{path}}` or `{{path | format}}` placeholders
- **`fields`** — top-level label-value pairs extracted from the payload via dot-notation paths
- **`items`** — optional repeating section for batch/list payloads (e.g., multiple profiles in one request)
- **Built-in formatters:** `currency` ($1,234.56), `date` (Mar 14, 2026), `number` (1,000,000), `truncate` (50 chars max)
- **Missing values** fall back to `"-"`

**Consumer override:** If the request already includes `metadata.display` at creation time, it takes precedence over the policy template. This lets consumers provide custom display data for edge cases.

#### Proposed Changes (Before / After)

Many approvals are not about a new thing but about a change to an existing one: a branch's transaction limit, a user's role, a product's price. For those, the reviewer needs to see what is there now next to what is being proposed. Put both snapshots in the payload and add a `changes` block to the template:

```json
{
  "title": "Update Branch {{current.code}}",
  "changes": {
    "current_path": "current",
    "proposed_path": "proposed",
    "fields": [
      { "label": "Name", "path": "name" },
      { "label": "Daily Limit", "path": "daily_limit", "format": "currency" },
      { "label": "Status", "path": "status" }
    ]
  }
}
```

With a payload such as:

```json
{
  "current":  { "code": "LAG-01", "name": "Lagos Island", "daily_limit": 500000, "status": "active" },
  "proposed": { "code": "LAG-01", "name": "Lagos Island", "daily_limit": 750000, "status": "inactive" }
}
```

Quorum resolves `metadata.display.changes` to one row per field with `from`, `to`, and a `changed` flag. The approval panel widget and the console render these as a before/after table with changed values highlighted, and show how many fields differ.

- **`current_path` / `proposed_path`** — dot-notation paths to the two snapshots inside the payload. Omit the current snapshot for a creation, or the proposed one for a deletion; the missing side renders as `"-"`.
- **`fields`** — optional. When omitted, Quorum compares every leaf attribute found in either snapshot, sorted by path, with labels derived from the path (`billing.account_id` becomes `Billing / Account Id`). List fields explicitly to control order, labels, and formatting.
- **`changed`** compares the raw values, not the formatted strings.
- `changes` can be combined with `title`, `fields`, and `items` in the same template.

Your webhook consumer receives the full request, so on approval it can read the `proposed` snapshot from the payload and apply it. Quorum decides; your application performs the write.

### Webhook Delivery

Quorum uses a **transactional outbox** to guarantee webhook delivery. Outbox entries are written in the same database transaction as the status update, so no event is ever lost. A signal-driven delivery worker dispatches entries via HTTP POST with HMAC-SHA256 signatures (`X-Signature-256` header), retrying failures with exponential backoff. A configurable heartbeat (default 30s) acts as a safety net for missed signals.

**Events.** A webhook subscribes to one or more of:

| Event | When | Extra payload |
|-------|------|---------------|
| `review_requested` | A request starts waiting on a stage: on creation and on every stage advance. | `review` (see below) |
| `approved` | The final stage passed, or a [bypass rule](#bypass-rules) matched. | |
| `rejected` | A stage's rejection policy was met. | |
| `cancelled` | The maker cancelled the request. | |
| `expired` | The policy's `auto_expire_duration` elapsed. | |

Every delivery carries `event`, the full `request` (payload, metadata, maker attributes, current stage), the `approvals` so far, and a `timestamp`.

#### Telling reviewers

Quorum has no user directory, so it cannot email the branch manager itself. What it can do is work out exactly who is allowed to act and hand that to your application, which does know how to reach people. That is the `review_requested` event:

```json
{
  "event": "review_requested",
  "request": { "id": "…", "type": "branch.update", "status": "pending", "current_stage": 1, "maker_id": "alice", "…": "…" },
  "review": {
    "reason": "stage_advanced",
    "stage_index": 1,
    "stage_name": "Receiving branch or head office",
    "total_stages": 2,
    "required_approvals": 1,
    "allowed_roles": ["branch_manager"],
    "allowed_permissions": ["approve_transfer"],
    "authorization_mode": "all",
    "scope": [
      { "attribute": "branch_id", "values": ["ABJ-02", "HQ"] }
    ],
    "eligible_reviewers": [],
    "excluded_user_ids": ["alice"],
    "expires_at": "2026-10-30T09:00:00Z"
  },
  "timestamp": "2026-10-29T09:00:00Z"
}
```

- **`reason`** is `created` for stage 0, `stage_advanced` for later stages, or `bypass_limit_exceeded` when a request that would have skipped review was held because of a [velocity limit](#bypass-rules).
- **`scope`** has its values already resolved for this request: the maker's attribute, the payload value, and any fixed values a rule names. A consumer does not need to know how scope rules work; it matches users whose `branch_id` is `ABJ-02` or `HQ`.
- **`excluded_user_ids`** lists people who cannot act even if they otherwise qualify: the maker, and anyone who has already voted on this stage.
- **`eligible_reviewers`**, when non-empty, is the per-request allowlist and overrides everything else.

The consumer's job is a directory query: users holding one of `allowed_roles` (or `allowed_permissions`, depending on `authorization_mode`), whose attributes match `scope`, minus `excluded_user_ids`. Then notify them by whatever channel suits, with a link to the request. The event is written in the same transaction as the request or the stage advance, so a reviewer is never left unnotified because the process died in between.

### Health Endpoint

The `/health` endpoint returns component-level health status (HTTP 200 when healthy, 503 when unhealthy) and does not require authentication. Use it for load balancer health checks and container orchestration readiness probes.

### Docker

**Docker Compose (recommended)** — starts PostgreSQL, runs migrations, and launches Quorum with the admin console and widgets in one command:

```bash
docker compose up
```

The server container includes a built-in healthcheck that polls `/health` every 10 seconds. Dependent services (like the seed/setup containers) wait for the server to be healthy before starting.

To start only the database (useful during local development):

```bash
docker compose up -d postgres
```

**Full demo with sample apps:**

```bash
docker compose --profile demo up
```

This starts Quorum, PostgreSQL, runs migrations, seeds sample data, and launches three sample consumer applications. See [Demo & Sample Apps](#demo--sample-apps) for details.

**Seed data only (no sample apps):**

```bash
docker compose --profile seed up
```

**Manual Docker builds:**

```bash
# API only (no console, no widgets)
docker build -t quorum .

# With admin console
docker build -f Dockerfile.console -t quorum-console .

# With admin console and embeddable widgets
docker build -f Dockerfile.all -t quorum-all .

# Run
docker run -p 8080:8080 -v /path/to/config.yaml:/etc/quorum/config.yaml quorum-all
```

---

## Demo & Sample Apps

### Seed Data

Quorum ships with a seed script that populates a fresh instance with sample policies, requests, and approvals:

```bash
# Against a running Quorum server
make seed

# Or via Docker Compose
docker compose --profile seed up
```

The seed script creates:
- **4 policies**: Expense Approval, Wire Transfer, System Access, Account Closure
- **6 requests** by different makers (alice, bob, charlie, dave, eve, frank)
- **Sample approvals**: 2 approved, 1 advanced to stage 2, 3 pending
- **1 webhook** (httpbin.org for testing)
- **Console admin**: `admin` / `admin123`

### Sample Consumer Applications

Three example apps in `examples/` demonstrate how real applications integrate with Quorum, each using a different tech stack:

| App | Port | Tech | Scenario |
|-----|------|------|----------|
| [Banking](examples/banking/) | [localhost:3001](http://localhost:3001) | Go + html/template | Multi-stage wire transfer approval with webhook callbacks |
| [Expense Tracker](examples/expenses/) | [localhost:3002](http://localhost:3002) | Node.js + Express | Single-stage expense approval with status polling |
| [Access Portal](examples/access-portal/) | [localhost:3003](http://localhost:3003) | Python + Flask | Threshold-based security review (2-of-3 voting) |

**Run the full demo:**

```bash
docker compose --profile demo up
```

This starts everything: PostgreSQL, Quorum with the admin console, seed data, and all three sample apps. Each app self-registers its policy with Quorum on startup.

**What each app demonstrates:**

- **Banking**: Webhook-driven flow — transfers execute automatically when Quorum sends an approved webhook with HMAC signature verification
- **Expense Tracker**: Polling-based flow — the detail page fetches the latest status from Quorum on each page load
- **Access Portal**: Threshold voting — 2 of 3 security reviewers must approve; not every rejection is fatal

Each app embeds all three Quorum widgets: `<quorum-request-list>` on the dashboard as an "Approval Queue", `<quorum-stage-progress>` on detail pages for stage visualization, and `<quorum-approval-panel>` for approve/reject actions. Clicking a row in the request list navigates to the matching local detail page.

Each app includes a **profile switcher** in the navigation bar for testing different personas (maker, manager, compliance officer, etc.) without restarting. Switch profiles to walk through the full approval flow: create a request as the maker, then switch to a reviewer to approve or reject it.

Visit the [Quorum console](http://localhost:8080/console/) (admin / admin123) to approve or reject requests created from the sample apps.

---

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development setup, project structure, Makefile targets, and frontend development workflow.

```bash
make test          # Unit tests
make lint          # Linter
make build-all     # Build with console + widgets
```

---

##  Community & Support

-  **Issues:** Report bugs or request features on [GitHub Issues](https://github.com/lawale/quorum/issues)
-  **Discussions:** Join [GitHub Discussions](https://github.com/lawale/quorum/discussions) for help, ideas, and general conversation
-  **Changelog:** See the [Releases](https://github.com/lawale/quorum/releases) page for updates

## License

Quorum is open-source software licensed under the [MIT License](LICENSE).
