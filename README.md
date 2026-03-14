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

[Quick Start](#quick-start) • [Key Features](#-key-features) • [Why Quorum?](#-why-quorum) • [Documentation](#documentation) • [Community & Support](#-community--support)

---

## ✨ Key Features

- **Uncouple Authorization from Your App** — Stop writing custom approval logic. Quorum acts as a centralized, reusable service for all your multi-stage workflows, keeping your core code clean and focused.

- **Dynamic, Multi-Stage Policies** — Define complex approval rules with JSON. Create sequential stages (e.g., "Manager," then "Compliance"), each with its own required approvals and rejection rules. Change them at runtime without redeploying.

- **Full Workflow Lifecycle** — Create requests that require one or more approvals. Support for approve, reject, and cancel operations, all recorded in a complete audit trail.

- **Layered Permission Guards** — Control exactly who can approve at every level:
  - **Self-approval prevention** — The maker of a request can never approve their own request.
  - **Eligible reviewers** — Optionally restrict each request to a specific set of allowed reviewers.
  - **Role-based stage access** — Each approval stage can define which roles are allowed to act (e.g., only `manager` for Stage 1, only `compliance_officer` for Stage 2).
  - **External permission checks** — Delegate to an external HTTP endpoint for custom business logic (e.g., "only managers in the same department can approve").
  - **Duplicate vote prevention** — A checker can only act once per stage.
  - **Request fingerprinting** — Configurable identity fields prevent duplicate pending requests for the same underlying resource.

- **Universal Integration** — Plug Quorum into any stack using its simple REST API. Get notified of state changes via HMAC-signed webhooks with configurable retries, ensuring reliable event delivery.

- **Optional Admin Console** — Manage everything with a built-in web UI built with Svelte 5. Browse requests, define policies, inspect audit logs, and manage operators — all served directly from the Go binary. Opt-in via build tag for a smaller default binary.

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
│ Webhook  │      │ Request  │      │ Layered  │      │  Checker │
│   or     │◀─────│ Resolved │◀─────│ Guards   │◀─────│ Approves │
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

- Go 1.24+
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
# PostgreSQL
migrate -path migrations/postgres -database "postgres://quorum:quorum@localhost:5432/quorum?sslmode=disable" up

# SQL Server
migrate -path migrations/mssql -database "sqlserver://sa:Password@localhost:1433?database=quorum" up
```

### 4. Start the Server

```bash
./bin/quorum -config config.yaml
```

The API is now live at `http://localhost:8080`. See the [API reference](#api-at-a-glance) below for your first request.

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
    "auto_expire_duration": "24h"
  }'
```

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

Your application, listening via webhook, receives a `request.approved` event and executes the transfer; safe in the knowledge that all checks have been passed.

---

## Permission Guards

Quorum enforces a layered permission model on every approval action. When a user attempts to approve or reject a request, each of these checks is evaluated in order:

| Guard | Scope | Description |
|-------|-------|-------------|
| **Self-approval prevention** | Built-in | The maker of a request cannot approve their own request. Always enforced. |
| **Eligible reviewers** | Per-request | An optional allowlist of user IDs set when creating the request. If provided, only those users can act. |
| **Allowed checker roles** | Per-stage | Each approval stage can restrict which roles are permitted. The checker must hold at least one matching role. |
| **Permission check URL** | Per-policy | An external HTTP endpoint called before each approval. The endpoint receives the full request context and returns `{"allowed": true/false}`. Use this for custom business logic your domain requires. |
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
│  External permission  │──▶ REJECT (endpoint returned allowed: false)
│  check (if configured)│
└───────┬───────────────┘
        │ pass
        ▼
   Vote recorded
```

### External Permission Check

When a policy has a `permission_check_url`, Quorum sends a POST request with this payload before allowing the approval:

```json
{
  "request_id": "uuid",
  "request_type": "wire_transfer",
  "checker_id": "bob",
  "checker_roles": ["manager"],
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
| `POST` | `/api/v1/policies` | Create a policy |
| `GET` | `/api/v1/policies` | List all policies |
| `GET` | `/api/v1/policies/{id}` | Get a policy by ID |
| `PUT` | `/api/v1/policies/{id}` | Update a policy |
| `DELETE` | `/api/v1/policies/{id}` | Delete a policy |
| `POST` | `/api/v1/webhooks` | Register a webhook |
| `GET` | `/api/v1/webhooks` | List webhooks |
| `DELETE` | `/api/v1/webhooks/{id}` | Delete a webhook |
| `GET` | `/health` | Health check (no auth) |
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
| `auth.mode` | `trust` | `trust`, `verify`, or `custom` |
| `webhook.max_retries` | `3` | Max webhook delivery attempts |
| `webhook.timeout` | `10s` | HTTP timeout per delivery attempt |
| `expiry.check_interval` | `1m` | How often to check for expired requests |
| `metrics.enabled` | `false` | Enable Prometheus metrics |
| `console.enabled` | `false` | Enable admin console API routes |
| `console.jwt_secret` | (auto) | JWT signing secret for console sessions |

### Authentication Modes

Quorum supports three authentication modes, configured via `auth.mode`:

- **`trust`** — Reads user identity from request headers (`X-User-ID`, `X-User-Roles`). Suitable when Quorum sits behind a trusted gateway or service mesh that sets these headers.
- **`verify`** — Validates JWT tokens against a JWKS endpoint. Extracts user ID and roles from configurable claims.
- **`custom`** — Delegates authentication to an external HTTP endpoint.

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

**Console features:** Dashboard overview, policy CRUD with multi-stage editor, webhook management, request browser with status/type filters, request detail with tabbed payload and audit trail, audit log search, and operator management.

### Docker

```bash
# Standard build (no console)
docker build -t quorum .

# With admin console
docker build -f Dockerfile.console -t quorum-console .

# Run
docker run -p 8080:8080 -v /path/to/config.yaml:/etc/quorum/config.yaml quorum
```

---

## Development

```bash
# Run tests
make test

# Lint
make lint

# Run the frontend dev server (with hot reload)
make console-dev

# Build everything (frontend + Go binary with console)
make build-console
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `build` | Build the Go binary (no console) |
| `build-console` | Build frontend + Go binary with console |
| `test` | Run all tests with race detector |
| `lint` | Run golangci-lint |
| `migrate-up` | Run PostgreSQL migrations |
| `migrate-down` | Roll back one PostgreSQL migration |
| `console-dev` | Start the Svelte dev server |
| `clean` | Remove build artifacts |

### Project Structure

```
cmd/server/          — Entry point
internal/
  auth/              — Authentication providers and permission checker
  config/            — Configuration loading
  metrics/           — Prometheus metrics
  model/             — Domain models
  server/            — HTTP handlers and routing
  service/           — Business logic and approval workflow
  store/             — Storage interfaces
    postgres/        — PostgreSQL implementation
    mssql/           — SQL Server implementation
  webhook/           — Webhook dispatcher with retries
console/
  console.go         — Embedded SPA handler (build tag: console)
  console_stub.go    — No-op when built without console tag
  frontend/          — Svelte 5 + TypeScript + Tailwind CSS
migrations/
  postgres/          — PostgreSQL migration files
  mssql/             — SQL Server migration files
```

---

## 🤝 Community & Support

- 🐛 **Issues:** Report bugs or request features on [GitHub Issues](https://github.com/lawale/quorum/issues)
- 💬 **Discussions:** Join [GitHub Discussions](https://github.com/lawale/quorum/discussions) for help, ideas, and general conversation
- 📖 **Changelog:** See the [Releases](https://github.com/lawale/quorum/releases) page for updates

## License

Quorum is open-source software licensed under the [MIT License](LICENSE).
