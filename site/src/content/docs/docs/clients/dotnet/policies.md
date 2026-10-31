---
title: Policies and webhook subscriptions
description: Managing a tenant's approval policies and webhook subscriptions from .NET.
---

Policies and webhook subscriptions are usually created once, by a deployment step or the [admin console](/docs/operate/console/), and the client covers that surface too. The records mirror the OpenAPI document (`PolicyInput`, `ApprovalStage`, `BypassRule`, `WebhookInput`), and the test suite fails if the server ever sends a field they lack.

## Policies

One policy per request type per tenant. `CreatePolicyAsync` registers it; a second policy for the same type is refused with `409`.

```csharp
var policy = await admin.CreatePolicyAsync(new PolicyInput
{
    Name = "Wire transfer",
    RequestType = "wire_transfer",
    Stages =
    [
        new ApprovalStage { Index = 0, Name = "Manager approval", RequiredApprovals = 1, AllowedCheckerRoles = ["manager"] },
        new ApprovalStage
        {
            Index = 1,
            Name = "Compliance review",
            RequiredApprovals = 2,
            AllowedCheckerRoles = ["compliance_officer"],
            Scope = [new ScopeRule { Attribute = "branch_id" }],
        },
    ],
    IdentityFields = ["source_account_id"],
    AutoExpireDuration = Duration.Format(TimeSpan.FromHours(24)),
});
```

`BypassRules` on the input take the same `BypassRule`, `BypassCondition` and `BypassLimit` records the server stores. Durations are Go strings such as `24h`; `Duration.Format` and `Duration.Parse` convert to and from `TimeSpan`. The returned `Policy` may carry `Warnings`: advice about the policy as saved, such as a scope attribute the server's authentication mode cannot supply. They did not block the save.

`UpdatePolicyAsync(id, new PolicyUpdate { ... })` changes a policy in place and applies to requests created from then on. A null property keeps its stored value; an empty list clears it. `GetPolicyAsync`, `ListPoliciesAsync(new Paging { ... })` and `DeletePolicyAsync` complete the set.

What each field does is covered under Concepts: [stages and guards](/docs/concepts/permission-guards/), [reviewer scoping](/docs/concepts/reviewer-scoping/), [bypass rules](/docs/concepts/bypass-rules/) and [display templates](/docs/concepts/display-templates/).

## Webhook subscriptions

A subscription names a URL, the events to deliver and the secret Quorum signs deliveries with. Keep the secret for [`Webhooks.Parse`](/docs/clients/dotnet/decisions/).

```csharp
var hook = await admin.CreateWebhookAsync(new WebhookInput
{
    Url = new Uri("https://app.example.com/webhooks/quorum"),
    Events = [WebhookEvent.Approved, WebhookEvent.Rejected, WebhookEvent.ReviewRequested],
    Secret = webhookSecret,
    RequestType = "wire_transfer",
});
```

`RequestType` is optional and restricts deliveries to one request type. `ListWebhooksAsync` returns subscriptions without their secrets, and `DeleteWebhookAsync` stops delivery.

## Pages

Every list call returns a `Page<T>` with `Data`, `Total` and the `Page` number. `PerPage` is capped at 100 by the server.
