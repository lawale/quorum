---
title: Policies and webhook subscriptions
description: Managing a tenant's approval policies and webhook subscriptions from Go.
---

Policies and webhook subscriptions are usually created once, by a deployment step or the [admin console](/docs/operate/console/), and the client covers that surface too. The types are the server's own (`PolicyInput`, `ApprovalStage`, `BypassRule`, `WebhookInput`), so a field the server accepts is a field the client has.

## Policies

One policy per request type per tenant. `CreatePolicy` registers it; a second policy for the same type is refused with `409`.

```go
policy, err := admin.CreatePolicy(ctx, client.PolicyInput{
    Name:        "Wire transfer",
    RequestType: "wire_transfer",
    Stages: []client.ApprovalStage{
        {Index: 0, Name: "Manager approval", RequiredApprovals: 1, AllowedCheckerRoles: []string{"manager"}},
        {
            Index:               1,
            Name:                "Compliance review",
            RequiredApprovals:   2,
            AllowedCheckerRoles: []string{"compliance_officer"},
            Scope:               []client.ScopeRule{{Attribute: "branch_id"}},
        },
    },
    IdentityFields:     []string{"source_account_id"},
    AutoExpireDuration: client.NewDuration(24 * time.Hour),
})
```

`BypassRules` on the input take the same `BypassRule`, `BypassCondition` and `BypassLimit` types the server stores. The returned `Policy` may carry `Warnings`: advice about the policy as saved, such as a scope attribute the server's authentication mode cannot supply. They did not block the save.

`UpdatePolicy(ctx, id, client.PolicyUpdate{...})` changes a policy in place and applies to requests created from then on. A field left at its zero value keeps its stored value; set a list field to an empty slice to clear it. `GetPolicy`, `ListPolicies(ctx, client.Paging{...})` and `DeletePolicy` complete the set.

What each field does is covered under Concepts: [stages and guards](/docs/concepts/permission-guards/), [reviewer scoping](/docs/concepts/reviewer-scoping/), [bypass rules](/docs/concepts/bypass-rules/) and [display templates](/docs/concepts/display-templates/).

## Webhook subscriptions

A subscription names a URL, the events to deliver and the secret Quorum signs deliveries with. Keep the secret for [`ParseWebhook`](/docs/clients/go/decisions/).

```go
hook, err := admin.CreateWebhook(ctx, client.WebhookInput{
    URL:    "https://app.example.com/webhooks/quorum",
    Events: []string{client.EventApproved, client.EventRejected, client.EventReviewRequested},
    Secret: webhookSecret,
})
```

`RequestType` on the input is optional and restricts deliveries to one request type. `ListWebhooks` returns subscriptions without their secrets, and `DeleteWebhook` stops delivery.

## Pages

Every list call returns a `Page[T]` with `Data`, `Total` and the `Page` number. `PerPage` is capped at 100 by the server.
