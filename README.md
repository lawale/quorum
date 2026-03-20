# Quorum: The Open-Source Approval Engine for Modern Systems

[![Go Report Card](https://goreportcard.com/badge/github.com/lawale/quorum)](https://goreportcard.com/report/github.com/lawale/quorum)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lawale/quorum)](https://go.dev/)

**An open-source, embeddable approval engine that treats authorization as a service, not a feature.**

Stop hardcoding "maker-checker" logic. Quorum is a standalone, self-hosted approval service that brings the [four-eyes principle](https://en.wikipedia.org/wiki/Two-man_rule) to any application. It provides a generic, policy-driven API for creating and managing multi-stage approval workflows, ensuring that critical actions are always reviewed before they take effect.

Think of it as a pluggable, external "approval board" for your entire infrastructure, from financial transactions and access requests to content publishing and infrastructure changes.

[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-316192?style=flat&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![SQL Server](https://img.shields.io/badge/SQL_Server-CC2927?style=flat&logo=microsoft-sql-server&logoColor=white)](https://www.microsoft.com/sql-server)
[![Prometheus](https://img.shields.io/badge/Prometheus-E6522C?style=flat&logo=prometheus&logoColor=white)](https://prometheus.io/)
[![Svelte](https://img.shields.io/badge/Svelte-FF3E00?style=flat&logo=svelte&logoColor=white)](https://svelte.dev/)

[Quick Start](#quick-start) • [Full Stack Local Setup](#running-the-full-stack-locally) • [Demo & Sample Apps](#demo--sample-apps) • [Key Features](#-key-features) • [Why Quorum?](#-why-quorum) • [Documentation](#documentation) • [Community & Support](#-community--support)

---

## ✨ Key Features

- **Uncouple Authorization from Your App** — Stop writing custom approval logic. Quorum acts as a centralized, reusable service for all your multi-stage workflows, keeping your core code clean and focused.

- **Dynamic, Multi-Stage Policies** — Define complex approval rules with JSON. Create sequential stages (e.g., "Manager," then "Compliance"), each with its own required approvals and rejection rules. Change them at runtime without redeploying.

- **Full Workflow Lifecycle** — Create requests that require one or more approvals. Support for approve, reject, and cancel operations, all recorded in a complete audit trail.

- **Layered Permission Guards** — Control exactly who can approve at every level:
  - **Self-approval prevention** — The maker of a request can never approve their own request.
  - **Eligible reviewers** — Optionally restrict each request to a specific set of allowed reviewers.
  - **Role-based stage access** — Each approval stage can define which roles are allowed to act (e.g., only `manager` for Stage 1, only `compliance_officer` for Stage 2).
  - **Permission-based stage access** — Each approval stage can define which permissions are required (e.g., only users with `approve_transfers` permission for Stage 2). When both roles and permissions are configured, `authorization_mode` controls whether either suffices (`"any"`) or both are required (`"all"`).
  - **External authorization checks** — Delegate to an external HTTP endpoint for custom business logic (e.g., "only managers in the same department can approve").
  - **Duplicate vote prevention** — A checker can only act once per stage.
  - **Request fingerprinting** — Configurable identity fields prevent duplicate pending requests for the same underlying resource.

- **Universal Integration** — Plug Quorum into any stack using its simple REST API. Get notified of state changes via HMAC-signed webhooks backed by a **transactional outbox**, webhook entries are written atomically with the status update, so no event is ever lost, even if the process crashes mid-request. A signal-driven delivery worker ensures near-instant dispatch with a configurable heartbeat as a safety net.

- **Optional Admin Console** — Manage everything with a built-in web UI built with Svelte 5. Browse requests, define policies, inspect audit logs, and manage operators — all served directly from the Go binary. Opt-in via build tag for a smaller default binary.

- **Embeddable UI Widgets** — Drop pre-built Web Components into your own application to give end users approval functionality without building custom UI. Three widgets (`<quorum-approval-panel>`, `<quorum-request-list>`, `<quorum-stage-progress>`) use Shadow DOM for style isolation and work with any frontend framework. CORS is enabled out of the box for cross-origin embedding. Opt-in via build tag.

- **Real-Time Updates via SSE** — Widgets receive instant push notifications the moment a request's state changes, via Server-Sent Events. No polling required, the widget opens a long-lived connection and the server pushes events on approve, reject, cancel, stage advance, and expiry. Polling is kept as an automatic fallback if the SSE connection drops.

- **Production-Ready from Day One:**
  - **Multi-Database Support** — Choose between PostgreSQL and Microsoft SQL Server.
  - **Flexible Authentication** — Support for header-based ("trust"), JWT/JWKS verification, or a custom auth endpoint to fit your existing security model.
  - **Observability** — Export key metrics (request volume, approval rates, webhook delivery) to Prometheus for monitoring and alerting.
  - **Audit-Ready** — Every action is permanently logged with actor, timestamp, and details, giving you a complete history for compliance (SOC2, ISO 27001, etc.).

---

## 💡 Why Quorum?

- **For Developers:** Stop reinventing the wheel. Integrate a powerful approval flow with a few API calls. Focus on your product, not approval boilerplate.

- **For Security & Compliance Teams:** Enforce a consistent "maker-checker" principle across your entire organization. Gain a centralized view of all pending and approved critical actions.

- **For Platform Engineers:** Provide a self-service approval engine for internal platforms, allowing other teams to add governance to their tools without bespoke development.

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

- Go 1.25+
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

- Go 1.25+
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

### Frontend Development

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

Your application, listening via webhook, receives a `request.approved` event and executes the transfer; safe in the knowledge that all checks have been passed. The webhook is guaranteed to be delivered as it was written to a durable outbox in the same database transaction as the status change, so nothing can be lost.

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
| **Dynamic authorization URL** | Per-policy | An external HTTP endpoint called before each approval. The endpoint receives the full request context and returns `{"allowed": true/false}`. Use this for custom business logic your domain requires. |
| **Duplicate vote prevention** | Per-stage | A checker can only vote once per stage. They may vote on different stages of the same request. |
| **Request fingerprinting** | Per-policy | Configurable `identity_fields` extracted from the payload and hashed. Prevents duplicate pending requests for the same resource. |

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

When a policy has a `dynamic_authorization_url`, Quorum sends a POST request with this payload before allowing the approval:

```json
{
  "request_id": "uuid",
  "request_type": "wire_transfer",
  "checker_id": "bob",
  "maker_id": "alice",
  "payload": { "source_account_id": "ACC-001", "amount": 50000 }
}
```

Your endpoint responds with:

```json
{ "allowed": true }
```

or

```json
{ "allowed": false, "reason": "Checker is in the same department as maker" }
```

This lets you implement arbitrary business rules: department separation, conflict-of-interest checks, rate limiting, and geographic restrictions, without modifying Quorum.

---

## API at a Glance

All endpoints are under `/api/v1` and require authentication (configurable via the `auth` section in config).

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/requests` | Create a new approval request |
| `GET` | `/api/v1/requests` | List requests (with `?status=`, `?type=`, `?page=`, `?per_page=` filters) |
| `GET` | `/api/v1/requests/{id}` | Get a request by ID |
| `POST` | `/api/v1/requests/{id}/approve` | Approve a request |
| `POST` | `/api/v1/requests/{id}/reject` | Reject a request |
| `POST` | `/api/v1/requests/{id}/cancel` | Cancel a request (maker only) |
| `GET` | `/api/v1/requests/{id}/audit` | Get audit trail for a request |
| `GET` | `/api/v1/requests/{id}/events` | SSE stream for real-time status updates |
| `POST` | `/api/v1/policies` | Create a policy |
| `GET` | `/api/v1/policies` | List policies (`?page=`, `?per_page=`) |
| `GET` | `/api/v1/policies/{id}` | Get a policy by ID |
| `PUT` | `/api/v1/policies/{id}` | Update a policy |
| `DELETE` | `/api/v1/policies/{id}` | Delete a policy |
| `POST` | `/api/v1/webhooks` | Register a webhook |
| `GET` | `/api/v1/webhooks` | List webhooks (`?page=`, `?per_page=`) |
| `DELETE` | `/api/v1/webhooks/{id}` | Delete a webhook |
| `GET` | `/health` | Health check with component status (no auth) |
| `GET` | `/metrics` | Prometheus metrics (no auth, when enabled) |

---

## Documentation

### Configuration

See [`config.example.yaml`](config.example.yaml) for all available options.

| Key | Default | Description |
|-----|---------|-------------|
| `server.host` | `0.0.0.0` | Listen address |
| `server.port` | `8080` | Listen port |
| `database.driver` | `postgres` | `postgres` or `mssql` |
| `auth.mode` | `trust` | `trust` (planned: `verify`, `custom`) |
| `webhook.max_retries` | `3` | Max webhook delivery attempts |
| `webhook.timeout` | `10s` | HTTP timeout per delivery attempt |
| `webhook.retry_interval` | `5s` | Base delay between retries (multiplied by attempt number) |
| `webhook.heartbeat` | `30s` | Safety-net polling interval for the outbox delivery worker |
| `expiry.check_interval` | `1m` | How often to check for expired requests |
| `metrics.enabled` | `false` | Enable Prometheus metrics |
| `console.enabled` | `false` | Enable admin console API routes |
| `console.jwt_secret` | (auto) | JWT signing secret for console sessions |
| `console.secure_cookies` | `false` | Set `true` in production (HTTPS) |
| `console.roles_url` | `""` | External endpoint returning JSON `[]string` of available roles |
| `console.permissions_url` | `""` | External endpoint returning JSON `[]string` of available permissions |

#### Environment Variable Overrides

Every config field can be overridden via environment variables using the `QUORUM_` prefix. Variable names are derived from the config path in SCREAMING_SNAKE_CASE:

| Env Var | Config Field |
|---------|-------------|
| `QUORUM_SERVER_HOST` | `server.host` |
| `QUORUM_SERVER_PORT` | `server.port` |
| `QUORUM_DATABASE_DRIVER` | `database.driver` |
| `QUORUM_DATABASE_HOST` | `database.host` |
| `QUORUM_DATABASE_PORT` | `database.port` |
| `QUORUM_DATABASE_USER` | `database.user` |
| `QUORUM_DATABASE_PASSWORD` | `database.password` |
| `QUORUM_DATABASE_NAME` | `database.name` |
| `QUORUM_DATABASE_MAX_OPEN_CONNS` | `database.max_open_conns` |
| `QUORUM_DATABASE_MAX_IDLE_CONNS` | `database.max_idle_conns` |
| `QUORUM_AUTH_MODE` | `auth.mode` |
| `QUORUM_WEBHOOK_MAX_RETRIES` | `webhook.max_retries` |
| `QUORUM_WEBHOOK_TIMEOUT` | `webhook.timeout` |
| `QUORUM_WEBHOOK_HEARTBEAT` | `webhook.heartbeat` |
| `QUORUM_EXPIRY_CHECK_INTERVAL` | `expiry.check_interval` |
| `QUORUM_METRICS_ENABLED` | `metrics.enabled` |
| `QUORUM_METRICS_PATH` | `metrics.path` |
| `QUORUM_CONSOLE_ENABLED` | `console.enabled` |
| `QUORUM_CONSOLE_JWT_SECRET` | `console.jwt_secret` |
| `QUORUM_CONSOLE_SECURE_COOKIES` | `console.secure_cookies` |
| `QUORUM_CONSOLE_ROLES_URL` | `console.roles_url` |
| `QUORUM_CONSOLE_PERMISSIONS_URL` | `console.permissions_url` |

**Precedence:** Environment variables take the highest priority, overriding both YAML config values and defaults. Duration values use Go syntax (e.g., `30s`, `5m`, `1h`). Invalid values are ignored with a warning log.

The config file path itself can be set via `QUORUM_CONFIG_PATH`, which overrides the `-config` CLI flag.

### Authentication Modes

Quorum supports pluggable authentication modes, configured via `auth.mode`:

- **`trust`** — Reads user identity from request headers (`X-User-ID`, `X-User-Roles`). Suitable when Quorum sits behind a trusted gateway or service mesh that sets these headers.
- **`verify`** *(planned)* — Will validate JWT tokens against a JWKS endpoint, extracting user ID and roles from configurable claims.
- **`custom`** *(planned)* — Will delegate authentication to an external HTTP endpoint.

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
  roles_url: "https://your-idp.example.com/api/roles"
  permissions_url: "https://your-idp.example.com/api/permissions"
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
npm install @quorum/embed
```

```typescript
// Registers all three custom elements
import '@quorum/embed';

// Optionally use the API client directly
import { createClient } from '@quorum/embed';

const client = createClient({
  apiUrl: 'https://your-quorum-host',
  token: 'Bearer ...',
});
const request = await client.getRequest('uuid-here');
```

**Usage:**

```html
<!-- Show approval details with approve/reject actions -->
<quorum-approval-panel
  request-id="uuid-here"
  api-url="https://your-quorum-host"
  token="Bearer ..."
></quorum-approval-panel>

<!-- List requests with filters and pagination -->
<quorum-request-list
  api-url="https://your-quorum-host"
  status="pending"
  page-size="10"
  token="Bearer ..."
></quorum-request-list>

<!-- Visualize approval stage progress (with real-time SSE updates) -->
<quorum-stage-progress
  request-id="uuid-here"
  api-url="https://your-quorum-host"
  token="Bearer ..."
  sse="true"
  poll-interval="30000"
></quorum-stage-progress>
```

**Authentication:** Pass a `token` attribute for Bearer authentication, or an `auth-headers` attribute (JSON string) for trust-mode headers like `{"X-User-ID": "alice", "X-User-Roles": "manager"}`.

**Real-time updates:** Both the approval panel and stage progress widgets connect via SSE by default for instant push notifications. Set `sse="false"` to disable and use polling only. The `poll-interval` attribute (default 30000ms) controls the fallback polling rate. When both widgets are on the same page, the stage progress widget also reacts to custom events dispatched by the approval panel for immediate cross-widget updates.

**Error handling:** Set `suppress-errors` on the widget to hide inline error messages. Listen for `quorum:error` events to handle errors externally (e.g., toast notifications). The event detail includes `{ action, message, status }` where `action` is `"load"`, `"approve"`, or `"reject"`.

**Events:** Widgets dispatch custom events you can listen for: `quorum:approved`, `quorum:rejected`, `quorum:select`, and `quorum:error`.

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

### Webhook Delivery

Quorum uses a **transactional outbox** to guarantee webhook delivery. When a request reaches a terminal status (approved, rejected, cancelled, expired), the outbox entries are written in the same database transaction as the status update. 

**How it works:**

1. A request reaches terminal status (e.g., approved).
2. In a single database transaction: the status is updated and outbox entries are created for each matching webhook and callback URL.
3. After the transaction commits, a signal wakes the delivery worker.
4. The worker reads pending entries from the outbox and delivers them via HTTP POST with HMAC-SHA256 signatures.
5. Successful deliveries are marked as delivered. Failures are retried with exponential backoff up to `max_retries`.

A configurable heartbeat (default 30s) acts as a safety net, even if a signal is missed, the worker will pick up pending entries on the next heartbeat tick.

**Webhook signature:** Every webhook request includes an `X-Signature-256` header containing `sha256=<hex>`, computed using HMAC-SHA256 with the webhook's secret. Verify this on your end to ensure the request came from Quorum.

### Health Endpoint

The `/health` endpoint returns component-level health status and does not require authentication. It checks all registered dependencies (database, and any future additions like Redis).

**Healthy response** (HTTP 200):

```json
{
  "status": "healthy",
  "components": {
    "postgres": { "status": "healthy" }
  }
}
```

**Unhealthy response** (HTTP 503):

```json
{
  "status": "unhealthy",
  "components": {
    "postgres": { "status": "unhealthy", "error": "connection refused" }
  }
}
```

Use this endpoint for load balancer health checks and container orchestration readiness probes.

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

```bash
# Run tests
make test

# Lint
make lint

# Run the console frontend dev server (with hot reload)
make console-dev

# Run the widgets frontend dev server
make embed-dev

# Build with console
make build-console

# Build with embeddable widgets
make build-embed

# Build with everything (console + widgets)
make build-all
```

### Makefile Targets

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

### Project Structure

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

---

## 🤝 Community & Support

- 🐛 **Issues:** Report bugs or request features on [GitHub Issues](https://github.com/lawale/quorum/issues)
- 💬 **Discussions:** Join [GitHub Discussions](https://github.com/lawale/quorum/discussions) for help, ideas, and general conversation
- 📖 **Changelog:** See the [Releases](https://github.com/lawale/quorum/releases) page for updates

## License

Quorum is open-source software licensed under the [MIT License](LICENSE).
