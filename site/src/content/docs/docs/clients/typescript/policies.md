---
title: Policies and webhook subscriptions
description: Managing a tenant's approval policies and webhook subscriptions from TypeScript.
---

Policies and webhook subscriptions are usually created once, by a deployment script or the [admin console](/docs/operate/console/), and the client covers that surface too. The shapes are the API's own, generated from the OpenAPI document, so field names are the snake_case ones the [REST API](/docs/integrate/api/) uses.

## Policies

One policy per request type per tenant. `createPolicy()` registers it; a second policy for the same type is refused with `409`.

```ts
const policy = await admin.createPolicy({
  name: "Wire transfer",
  request_type: "wire_transfer",
  stages: [
    { index: 0, name: "Manager approval", required_approvals: 1, allowed_checker_roles: ["manager"] },
    {
      index: 1,
      name: "Compliance review",
      required_approvals: 2,
      allowed_checker_roles: ["compliance_officer"],
      scope: [{ attribute: "branch_id" }],
    },
  ],
  identity_fields: ["source_account_id"],
  auto_expire_duration: "24h",
  bypass_rules: [
    {
      name: "petty cash",
      payload: [{ path: "amount", op: "lt", value: 1000 }],
      limits: [{ window: "24h", max_count: 3 }],
    },
  ],
});
```

The returned `Policy` may carry `warnings`: advice about the policy as saved, such as a scope attribute the server's authentication mode cannot supply. They did not block the save.

`updatePolicy(id, update)` changes a policy in place and applies to requests created from then on. A field left out of the update keeps its stored value; send an empty array to clear a list field. `getPolicy`, `listPolicies({ page, perPage })` and `deletePolicy` complete the set.

What each field does is covered under Concepts: [stages and guards](/docs/concepts/permission-guards/), [reviewer scoping](/docs/concepts/reviewer-scoping/), [bypass rules](/docs/concepts/bypass-rules/) and [display templates](/docs/concepts/display-templates/).

## Webhook subscriptions

A subscription names a URL, the events to deliver and the secret Quorum signs deliveries with. Keep the secret for [`parseWebhook()`](/docs/clients/typescript/decisions/).

```ts
await admin.createWebhook({
  url: "https://app.example.com/webhooks/quorum",
  events: ["approved", "rejected", "review_requested"],
  secret: webhookSecret,
  request_type: "wire_transfer",
});
```

`request_type` is optional and restricts deliveries to one request type. `listWebhooks({ page, perPage })` returns subscriptions without their secrets, and `deleteWebhook(id)` stops delivery.

## Pages

Every list call returns a `Page<T>` with `data`, `total` and the `page` number. `perPage` is capped at 100 by the server.
