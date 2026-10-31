---
title: REST API
description: The Quorum HTTP API for requests, approvals, policies and webhooks, with an OpenAPI contract served by every running instance.
sidebar:
  order: 1
---

All API endpoints are under `/api/v1` and require authentication. How the caller is identified depends on the [authentication mode](/docs/operate/authentication/): a bearer JWT in `verify` mode, or `X-User-ID` and companion headers in `trust` mode. Every call is scoped to a tenant.

The full contract is published as OpenAPI in the repository at [`api/openapi.yaml`](https://github.com/lawale/quorum/blob/master/api/openapi.yaml) and served by every Quorum at `/openapi.yaml`. The test suite validates real server traffic against it, so it is never out of date.

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/v1/requests` | Create a request. Send an `Idempotency-Key` header to make retries safe. |
| `GET` | `/api/v1/requests` | List requests, with status and type filters and pagination. |
| `GET` | `/api/v1/requests/{id}` | Get a request, including `viewer_can_act` for the caller. |
| `POST` | `/api/v1/requests/{id}/approve` | Record an approval vote on the current stage. |
| `POST` | `/api/v1/requests/{id}/reject` | Record a rejection vote on the current stage. |
| `POST` | `/api/v1/requests/{id}/cancel` | Cancel a pending request. Only the maker may cancel. |
| `GET` | `/api/v1/requests/{id}/audit` | The audit trail of a request. |
| `GET` | `/api/v1/requests/{id}/events` | Server-Sent Events stream of state changes for a request. |
| `POST` | `/api/v1/policies` | Create a policy. One policy per request type per tenant. |
| `GET` | `/api/v1/policies` | List policies. |
| `GET` | `/api/v1/policies/{id}` | Get a policy. |
| `PUT` | `/api/v1/policies/{id}` | Update a policy. Applies to requests created from then on. |
| `DELETE` | `/api/v1/policies/{id}` | Delete a policy. |
| `POST` | `/api/v1/webhooks` | Subscribe a webhook to one or more events. |
| `GET` | `/api/v1/webhooks` | List webhooks. |
| `DELETE` | `/api/v1/webhooks/{id}` | Delete a webhook. |
| `GET` | `/health` | Component health and the running version. No authentication. |

## The request lifecycle in three calls

The [wire transfer walkthrough](/docs/getting-started/walkthrough/) shows the lifecycle end to end. In short:

```bash
# Maker creates
curl -X POST http://localhost:8080/api/v1/requests \
  -H "Content-Type: application/json" -H "X-User-ID: alice" \
  -d '{"type": "wire_transfer", "payload": {"amount": 50000, "destination": "IBAN-12345"}}'

# Checker approves
curl -X POST http://localhost:8080/api/v1/requests/{id}/approve \
  -H "X-User-ID: bob" -H "X-User-Roles: manager" \
  -d '{"comment": "Funds available."}'

# Anyone in the tenant reads the trail
curl http://localhost:8080/api/v1/requests/{id}/audit -H "X-User-ID: auditor"
```

## Errors

A decision that fails a [permission guard](/docs/concepts/permission-guards/) returns `403`. A request for a resource that does not exist in the caller's tenant returns `404`, so one tenant cannot discover another's IDs. A duplicate pending request for the same fingerprint returns `409`, as does a vote on a request that is no longer pending or a second vote by the same checker on one stage. Validation failures return `400` with a message naming the field.

## Clients

Rather than calling the API by hand, use the [Go client](/docs/integrate/go-client/), which shares the server's own types, or the [`@oluwavader/quorum-embed`](/docs/integrate/widgets/#javascript-api) package, which exposes a small browser client alongside the widgets.
