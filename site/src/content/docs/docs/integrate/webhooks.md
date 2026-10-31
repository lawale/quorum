---
title: Webhooks
description: Guaranteed webhook delivery from a transactional outbox, signed with HMAC-SHA256, including the review_requested event that tells your app who may act next.
sidebar:
  order: 5
---

Quorum uses a **transactional outbox** to guarantee webhook delivery. Outbox entries are written in the same database transaction as the status update, so no event is ever lost. A signal-driven delivery worker dispatches entries via HTTP POST with HMAC-SHA256 signatures in the `X-Signature-256` header, retrying failures with exponential backoff. A configurable heartbeat (default 30s) acts as a safety net for missed signals.

Subscribe with `POST /api/v1/webhooks`, giving a URL, a secret and the events you want. The [Go client](/docs/integrate/go-client/) verifies and decodes deliveries with `client.ParseWebhook`, and the [TypeScript client](/docs/integrate/typescript-client/) with `parseWebhook`.

## Events

A webhook subscribes to one or more of:

| Event | When | Extra payload |
|-------|------|---------------|
| `review_requested` | A request starts waiting on a stage: on creation and on every stage advance. | `review` (see below) |
| `approved` | The final stage passed, or a [bypass rule](/docs/concepts/bypass-rules/) matched. | |
| `rejected` | A stage's rejection policy was met. | |
| `cancelled` | The maker cancelled the request. | |
| `expired` | The policy's `auto_expire_duration` elapsed. | |

Every delivery carries `event`, the full `request` (payload, metadata, maker attributes, current stage), the `approvals` so far, and a `timestamp`.

## Telling reviewers

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

- **`reason`** is `created` for stage 0, `stage_advanced` for later stages, or `bypass_limit_exceeded` when a request that would have skipped review was held because of a [velocity limit](/docs/concepts/bypass-rules/#velocity-limits).
- **`scope`** has its values already resolved for this request: the maker's attribute, the payload value, and any fixed values a rule names. A consumer does not need to know how scope rules work; it matches users whose `branch_id` is `ABJ-02` or `HQ`.
- **`excluded_user_ids`** lists people who cannot act even if they otherwise qualify. Today that is the maker; the list exists so consumers need no special case when more exclusions are added.
- **`eligible_reviewers`**, when non-empty, is the per-request allowlist and overrides everything else.

The consumer's job is a directory query: users holding one of `allowed_roles` (or `allowed_permissions`, depending on `authorization_mode`), whose attributes match `scope`, minus `excluded_user_ids`. Then notify them by whatever channel suits, with a link to the request. The event is written in the same transaction as the request or the stage advance, so a reviewer is never left unnotified because the process died in between.

## Delivery settings

Retries, timeouts and the heartbeat interval are set under `webhook` in the [configuration](/docs/operate/configuration/). The admin console shows every delivery attempt with its status, and a per-webhook delivery rate.
