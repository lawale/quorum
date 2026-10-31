---
title: MediatR
description: Approve MediatR commands with an attribute. Quorum holds a marked command until it is approved, then the same command runs through the same handler from the webhook.
---

[`Quorum.Client.MediatR`](https://www.nuget.org/packages/Quorum.Client.MediatR) is for applications whose writes are MediatR commands. Mark a command, and it is approved before it runs, with no change to its handler. The checker approves exactly the object that executes.

```bash
dotnet add package Quorum.Client.MediatR
```

## Setup

```csharp
[RequiresApproval("wire_transfer")]
public record SubmitWireTransfer(Guid Account, decimal Amount, string Iban) : IRequest<TransferResult>;
```

```csharp
builder.Services.AddQuorumClient(o => { o.BaseAddress = new Uri("https://quorum.example.com"); o.Tenant = "banking"; });
builder.Services.AddMediatR(cfg => cfg.RegisterServicesFromAssembly(typeof(Program).Assembly));
builder.Services.AddQuorumApprovals(o => o.WebhookSecret = builder.Configuration["Quorum:WebhookSecret"]);
builder.Services.AddScoped<IQuorumIdentityProvider, CurrentUserIdentity>();

app.MapPost("/quorum/webhook", async (HttpRequest http, ApprovalDispatcher dispatcher, CancellationToken ct) =>
{
    try
    {
        await dispatcher.DispatchAsync(http.Body, http.Headers[Webhooks.SignatureHeader], ct);
        return Results.NoContent();
    }
    catch (InvalidSignatureException) { return Results.Unauthorized(); }
});
```

Submit the command instead of sending it:

```csharp
app.MapPost("/transfers", async (SubmitWireTransfer command, Approvals approvals) =>
{
    var request = await approvals.SubmitAsync(command);
    return Results.Accepted($"/transfers/{request.Id}", request);
});
```

`SubmitAsync` returns the Quorum request. It is pending when checkers must act, or already approved when a [bypass rule](/docs/concepts/bypass-rules/) matched. Either way the handler has not run yet. It runs when Quorum delivers the `approved` webhook: the dispatcher decodes the command from the request payload and sends it through MediatR in a fresh scope. A `rejected`, `cancelled` or `expired` delivery runs nothing.

Execution has one path, so whatever follows a decision lives in one place: handle the `RequestDecided` notification, which carries the request, the decoded command and, for approvals, the handler's result. Sending a marked command directly with `ISender.Send` throws `ApprovalRequiredException`, so approval cannot be skipped by accident.

## The pieces

- **`[RequiresApproval]`** marks a command. The Quorum request type defaults to the class name in snake_case, so `SubmitWireTransfer` selects the `submit_wire_transfer` policy; pass a name to override it.
- **`IQuorumIdentityProvider`** supplies the maker's credentials, typically from the current HTTP user. Register it as scoped. Without one, the client's default credentials are used.
- **`IIdempotentCommand`** lets a command carry an idempotency key, so a retried submission returns the request opened the first time.
- **`Approvals`** submits a marked command and returns the Quorum request.
- **`IApprovalContext`** is injectable into handlers. Its `Request` is the approved request the handler is executing, with the maker, the approvals and the request ID. For commands that need no approval it is null.
- **`ApprovalDispatcher`** is the webhook side. `DispatchAsync(Stream, signature)` verifies and dispatches; `DispatchAsync(WebhookPayload)` takes a delivery you verified yourself. Both return a `DispatchResult` with the outcome, the request and the handler's result.

## Setting it up in Quorum

Create a policy whose request type matches each marked command, and a webhook subscription for your endpoint carrying the same secret as `WebhookSecret`. Both can be done from the [client](/docs/clients/dotnet/policies/) or the console. Bypass rules, reviewer scoping, stages and expiry are all policy configuration; nothing in the application changes when they do.

## Delivery semantics

Quorum retries a delivery until your endpoint answers 2xx, so a handler that throws will be retried. Deliveries are at least once, so handlers should be idempotent against the request ID.

The delivery is queued in the same transaction that creates the request, so for a bypassed command the handler can run before `SubmitAsync` has returned, and on a different instance of your application. Write your code to that ordering:

- The command carries everything the handler needs. Nothing the submitter writes after `SubmitAsync` is guaranteed to be visible to the handler.
- If the submitter must record the attempt itself, it generates an idempotency key first, writes its record under that key, then submits the command as an `IIdempotentCommand`. The handler finds the record through `IApprovalContext.Request.IdempotencyKey`.
- After `SubmitAsync`, the submitter's only job is to hand the request ID back. What follows a decision belongs in the handler or in a `RequestDecided` handler.

## MediatR licensing

MediatR 13 and later require a license key for commercial use above the publisher's revenue threshold. This package only consumes the `IPipelineBehavior` and `ISender` abstractions; the license is between you and the MediatR authors.
