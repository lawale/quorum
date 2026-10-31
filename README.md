# Quorum

[![CI](https://github.com/lawale/quorum/actions/workflows/ci.yml/badge.svg)](https://github.com/lawale/quorum/actions/workflows/ci.yml)
[![Security](https://github.com/lawale/quorum/actions/workflows/security.yml/badge.svg)](https://github.com/lawale/quorum/actions/workflows/security.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/lawale/quorum)](https://goreportcard.com/report/github.com/lawale/quorum)
[![npm](https://img.shields.io/npm/v/@oluwavader/quorum-embed)](https://www.npmjs.com/package/@oluwavader/quorum-embed)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lawale/quorum)](https://go.dev/)

**Every serious application eventually needs "someone else has to sign off on this". Quorum is that someone else, as a service.**

Wire transfers, production access, price changes, content going live: the [four-eyes principle](https://en.wikipedia.org/wiki/Two-man_rule) shows up everywhere, and every team rebuilds it from scratch. Quorum pulls that logic out of your applications into one self-hosted service. Your app creates a request; Quorum matches it to a policy, walks it through the stages you defined, enforces who may act at each one, records everything, and tells you when it is decided. Rules live in data and change without a deploy. The approval UI is included as Web Components you drop into any front end.

**Website and documentation:** [quorum.olawalelawal.dev](https://quorum.olawalelawal.dev)
**Live demo:** [banking](https://quorum-bank-demo.olawalelawal.dev) · [expenses](https://quorum-expense-demo.olawalelawal.dev) · [access portal](https://quorum-access-portal-demo.olawalelawal.dev)

## What you get

- **Policies as data.** Sequential stages, each with its own approval threshold, allowed roles and permissions, and rejection rule. Edit them at runtime in the console or the API.
- **Layered permission guards.** Self-approval prevention, per-request eligible reviewers, per-stage roles and permissions, [reviewer scoping](https://quorum.olawalelawal.dev/docs/concepts/reviewer-scoping/) by attributes such as branch, a dynamic authorization URL, and one vote per checker per stage.
- **Bypass rules with velocity limits.** Declare what needs no review, and stop a threshold from being gamed. [Details](https://quorum.olawalelawal.dev/docs/concepts/bypass-rules/).
- **Guaranteed webhooks.** Written to a transactional outbox with the state change, signed with HMAC-SHA256, retried with backoff. A `review_requested` event tells your app exactly who may act next.
- **Embeddable widgets and an admin console.** Three Lit Web Components with live updates over SSE, plus an opt-in console for policies, requests, audit and deliveries.
- **Your identity provider stays in charge.** Validate JWTs against any OIDC provider's JWKS, or trust headers from your gateway. PostgreSQL or SQL Server, Prometheus metrics, audit log, Docker images.

| Requests dashboard | Approval detail |
|---|---|
| ![Dashboard](docs/screenshots/console-dashboard.png) | ![Approval detail](docs/screenshots/console-approval-detail.png) |

## Quick start

```bash
docker compose up            # PostgreSQL, migrations, Quorum with console and widgets
```

Then open [localhost:8080/console/](http://localhost:8080/console/), or create a policy and a request:

```bash
curl -X POST localhost:8080/api/v1/policies -H "X-User-ID: admin" -H "Content-Type: application/json" -d '{
  "name": "Wire transfer", "request_type": "wire_transfer",
  "stages": [{ "index": 0, "name": "Manager", "required_approvals": 1,
               "allowed_checker_roles": ["manager"], "rejection_policy": "any" }]
}'

curl -X POST localhost:8080/api/v1/requests -H "X-User-ID: alice" -H "Content-Type: application/json" \
  -d '{"type": "wire_transfer", "payload": {"amount": 50000, "destination": "IBAN-12345"}}'

curl -X POST localhost:8080/api/v1/requests/{id}/approve -H "X-User-ID: bob" -H "X-User-Roles: manager"
```

The [quick start guide](https://quorum.olawalelawal.dev/docs/getting-started/quickstart/) covers building from source, and the [walkthrough](https://quorum.olawalelawal.dev/docs/getting-started/walkthrough/) takes a two-stage transfer from policy to webhook.

## Integrate

- [REST API](https://quorum.olawalelawal.dev/docs/integrate/api/), with the contract in [`api/openapi.yaml`](api/openapi.yaml) and validated against the real server in tests.
- [Go client](https://quorum.olawalelawal.dev/docs/integrate/go-client/): `github.com/lawale/quorum/clients/go`.
- [TypeScript client](https://quorum.olawalelawal.dev/docs/integrate/typescript-client/): `npm install @oluwavader/quorum-client`, for Node and the browser.
- [.NET client](https://quorum.olawalelawal.dev/docs/integrate/dotnet-client/): `dotnet add package Quorum.Client`, with `AddQuorumClient` for ASP.NET Core, and `Quorum.Client.MediatR` to approve MediatR commands with an attribute.
- [Widgets](https://quorum.olawalelawal.dev/docs/integrate/widgets/): `npm install @oluwavader/quorum-embed`, or a script tag from your Quorum server.
- [Webhooks](https://quorum.olawalelawal.dev/docs/integrate/webhooks/).

## Repository layout

| Path | Contents |
|------|----------|
| `cmd/`, `internal/` | The server |
| `api/` | OpenAPI contract |
| `clients/` | Official clients |
| `console/`, `widgets/` | Admin console (Svelte) and embeddable widgets (Lit) |
| `examples/` | Sample consumer apps: banking (Go), expenses (Node.js), access portal (Python) |
| `migrations/` | PostgreSQL and SQL Server migrations |
| `site/` | Website and documentation (Astro Starlight) |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development setup, project structure and Makefile targets.

```bash
make test          # Unit tests
make lint          # Linter
make build-all     # Build with console + widgets
```

Bugs and feature requests go to [GitHub Issues](https://github.com/lawale/quorum/issues); questions and ideas to [Discussions](https://github.com/lawale/quorum/discussions). Changes are listed on the [Releases](https://github.com/lawale/quorum/releases) page.

## License

Quorum is open-source software licensed under the [MIT License](LICENSE).
