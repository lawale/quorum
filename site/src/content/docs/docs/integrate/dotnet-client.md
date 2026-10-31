---
title: .NET client
description: The official .NET client for Quorum, with per-call identity, ASP.NET Core registration, webhook signature verification and waiting on a decision.
sidebar:
  order: 4
---

[`Quorum.Client`](https://www.nuget.org/packages/Quorum.Client) is the .NET client. It targets .NET 8 and .NET 10 and depends only on the BCL and `Microsoft.Extensions.Http`.

```bash
dotnet add package Quorum.Client
```

Its models are records that mirror the [OpenAPI document](/docs/integrate/api/). The test suite drives the real server with unmapped members disallowed, so a field the server sends that the client lacks fails the build rather than vanishing. On top of the API the client adds what REST cannot express: per-call identity, idempotent creation, webhook signature verification, an event stream, and waiting on a decision.

## Usage

A `QuorumClient` talks to one Quorum server. `As()` returns a client that acts as the given identity and leaves the original unchanged, so one instance serves every maker and checker in a process.

```csharp
using Quorum.Client;

var quorum = new QuorumClient(new Uri("https://quorum.example.com"), o => o.Tenant = "banking");

var req = await quorum.As(new Identity { UserId = "alice" }).CreateRequestAsync(new CreateRequest
{
    Type = "wire_transfer",
    Payload = transfer,
    IdempotencyKey = $"transfer-{transfer.Id}",
});

try
{
    await quorum.As(new Identity { UserId = "bob", Roles = ["manager"] }).ApproveAsync(req.Id, "looks right");
}
catch (QuorumException e) when (e.IsForbidden) { /* bob may not act on this request */ }

using var cts = new CancellationTokenSource(TimeSpan.FromMinutes(1));
var final = await quorum.As(new Identity { UserId = "alice" }).WaitForDecisionAsync(req.Id, cancellationToken: cts.Token);

// Pass the raw body: re-serialised JSON will not verify.
try
{
    var payload = await Webhooks.ParseAsync(Request.Body, Request.Headers[Webhooks.SignatureHeader], webhookSecret);
}
catch (InvalidSignatureException) { /* not from Quorum */ }
```

Every non-2xx response throws a `QuorumException` carrying the status, the server's message and the call that failed, with `IsNotFound`, `IsConflict`, `IsForbidden`, `IsUnauthorized` and `IsBadRequest` for the usual branches.

## ASP.NET Core

```csharp
builder.Services.AddQuorumClient(o =>
{
    o.BaseAddress = new Uri(builder.Configuration["Quorum:Url"]!);
    o.Tenant = "banking";
});
```

Inject `QuorumClient` and call `As(...)` with the current user's identity per request. The registration goes through `IHttpClientFactory` with an unbounded `HttpClient` timeout so event streams are not cut off; the per-call timeout in the options still applies.

## MediatR: approval without integration code

[`Quorum.Client.MediatR`](https://www.nuget.org/packages/Quorum.Client.MediatR) is for applications whose writes are MediatR commands. Mark a command, and it is approved before it runs, with no change to its handler.

```csharp
[RequiresApproval("wire_transfer")]
public record SubmitWireTransfer(Guid Account, decimal Amount, string Iban) : IRequest<TransferResult>;
```

```csharp
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

```csharp
var request = await approvals.SubmitAsync(new SubmitWireTransfer(account, 5000m, iban));
return Results.Accepted($"/transfers/{request.Id}", request);
```

`SubmitAsync` returns the Quorum request, pending or already approved by a bypass rule. The handler runs later, when the `approved` webhook arrives: the dispatcher decodes the command from the request payload and sends it through MediatR, and the pipeline lets it through to the handler. The checker approved exactly the object that executes, and there is one execution path, so what follows a decision is written once, in a `RequestDecided` handler. Sending a marked command directly throws, so approval cannot be skipped by accident.

A rejected, cancelled or expired delivery runs nothing. Handlers that want the maker or the approvals inject `IApprovalContext`. Policies, stages, scoping and bypass rules are configured in Quorum against the command's request type, which defaults to the class name in snake_case.

## Authentication

Against a server in `verify` mode, pass `new BearerToken(jwt)` to `As()`. Against one in `trust` mode, pass an `Identity` (`UserId`, `Roles`, `Permissions`, `TenantId`, `Attributes`); it is sent as gateway headers, so only do this from code that is the trusted gateway or sits behind one. If the server reads identity from renamed headers, or attribute headers such as `X-User-Branch`, set `TrustHeaders`. See [Authentication modes](/docs/operate/authentication/).

## Payloads, events and webhooks

`CreateRequest.Payload` is any object, serialised with web defaults (camelCase) unless you set `PayloadSerializerOptions`; match it to how your policies' identity fields and bypass paths are written. `Request.PayloadAs<T>()` decodes it again.

`EventsAsync(id)` is an `IAsyncEnumerable` over the request's Server-Sent Events. It ends when the request is terminal; an `updated` event means the request changed and should be re-read. `WaitForDecisionAsync` is built on it.

`Webhooks.Parse` and `Webhooks.ParseAsync` verify the `X-Signature-256` header with a constant-time HMAC-SHA256 check and decode the payload. Give them the body exactly as received: re-serialised JSON will not verify. See [Webhooks](/docs/integrate/webhooks/).

## Where to look next

- The [package README](https://github.com/lawale/quorum/tree/master/clients/dotnet) covers the full surface, including policies and webhooks, and how the tests run against the real server. The [MediatR package README](https://github.com/lawale/quorum/tree/master/clients/dotnet/src/Quorum.Client.MediatR) covers delivery semantics and idempotency.
- The client reports the server's release version from `HealthAsync()`, and its own as `QuorumClient.Version`, so you can confirm which Quorum and which client a deployment is running.
