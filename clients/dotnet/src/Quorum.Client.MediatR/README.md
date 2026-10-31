# Quorum.Client.MediatR

Routes MediatR commands through [Quorum](https://quorum.olawalelawal.dev) for approval. Mark a command, and sending it opens an approval request instead of running its handler. When a checker approves, the same command object is dispatched to the same handler. The handler never learns that approval exists, and the checker approves exactly what will execute.

```bash
dotnet add package Quorum.Client.MediatR
```

## Usage

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

Sending the command now has three outcomes:

```csharp
try
{
    var result = await sender.Send(new SubmitWireTransfer(account, 5000m, iban));
    // A bypass rule matched: Quorum approved it on creation and the handler ran.
}
catch (ApprovalPendingException e)
{
    return Results.Accepted($"/transfers/pending/{e.Request.Id}");
}
```

The third outcome happens later. Quorum delivers an `approved` webhook, the dispatcher decodes the command from the request payload, and sends it through MediatR in a fresh scope. `ApprovalBehavior` recognises the replay and lets it through to the handler. A `rejected`, `cancelled` or `expired` delivery does not run anything.

Every terminal delivery publishes a `RequestDecided` notification carrying the request, the decoded command, and the handler's result for approved requests. Handle it to notify the maker or to record the outcome.

## Pieces

- **`[RequiresApproval]`** marks a command. The Quorum request type defaults to the class name in snake_case, so `SubmitWireTransfer` selects the `submit_wire_transfer` policy; pass a name to override it.
- **`IQuorumIdentityProvider`** supplies the maker's credentials, typically from the current HTTP user. Register it as scoped. Without one, the client's default credentials are used.
- **`IIdempotentCommand`** lets a command carry an idempotency key, so a retried submission returns the request opened the first time.
- **`IApprovalContext`** is injectable into handlers. During a replay its `Request` is the approved request, with the maker, the approvals and the request ID. For a bypassed command it is the auto-approved request. For commands that need no approval it is null.
- **`ApprovalDispatcher`** is the webhook side. `DispatchAsync(Stream, signature)` verifies and dispatches; `DispatchAsync(WebhookPayload)` takes a delivery you verified yourself.

## Setting it up in Quorum

Create a policy whose request type matches each marked command, and a webhook subscription for your endpoint carrying the same secret as `WebhookSecret`. Bypass rules, reviewer scoping, stages and expiry are all policy configuration; nothing in the application changes when they do.

## Delivery semantics

Quorum retries a delivery until your endpoint answers 2xx, so a handler that throws will be retried. Deliveries are at least once, so handlers should be idempotent against the request ID. A command that a bypass rule approved runs inline at submission; Quorum still sends an `approved` delivery for it, which the dispatcher recognises by the absence of approvals and ignores.

## MediatR licensing

MediatR 13 and later require a license key for commercial use above the publisher's revenue threshold. This package only consumes the `IPipelineBehavior` and `ISender` abstractions; the license is between you and the MediatR authors.
