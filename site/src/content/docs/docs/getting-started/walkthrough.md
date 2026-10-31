---
title: "Walkthrough: securing a wire transfer"
description: Enforce a two-stage, maker-checker approval for a high-value wire transfer with Quorum, from policy to webhook, so no single person can act alone.
sidebar:
  order: 4
---

This example shows how Quorum enforces a two-stage approval for a high-value wire transfer, preventing a single person from acting alone. It assumes a server running in the default `trust` [authentication mode](/docs/operate/authentication/), where identity comes from request headers.

## 1. Define the policy (one-time setup)

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

The `display_template` tells Quorum how to render the payload for reviewers. See [Display templates](/docs/concepts/display-templates/) for details. The `identity_fields` are hashed into a fingerprint so a second pending request for the same account is rejected as a duplicate.

## 2. The maker creates a request

A bank teller, `alice`, initiates a transfer. The request is now **pending**:

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

## 3. A checker approves the request

A manager, `bob`, approves. The request moves to the "Compliance Review" stage. Later, two compliance officers, `charlie` and `dave`, also approve, and the request is automatically marked as **approved**:

```bash
curl -X POST http://localhost:8080/api/v1/requests/{request_id}/approve \
  -H "X-User-ID: bob" \
  -H "X-User-Roles: manager" \
  -d '{"comment": "Funds available, looks good."}'
```

If `alice` tried to approve her own request, or `bob` tried to vote again on the compliance stage, the call would return `403`. Every check that runs before a vote is recorded is listed under [Permission guards](/docs/concepts/permission-guards/).

## 4. The system acts on the result

Your application, listening via [webhook](/docs/integrate/webhooks/), receives an `approved` event and executes the transfer, safe in the knowledge that all checks have passed. The webhook is guaranteed to be delivered because it was written to a durable outbox in the same database transaction as the status change, so nothing can be lost.

The same flow in Go, using the [Go client](/docs/integrate/go-client/), is what the [banking sample app](/docs/demo/) runs. The [TypeScript](/docs/integrate/typescript-client/) and [.NET](/docs/integrate/dotnet-client/) clients offer the same calls for their platforms.
