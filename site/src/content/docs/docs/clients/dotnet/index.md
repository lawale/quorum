---
title: .NET client
description: The official .NET client for Quorum, with per-call identity, ASP.NET Core registration, a MediatR integration, webhook signature verification and waiting on a decision.
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
```

The rest of the request surface follows the API one to one: `GetRequestAsync`, `ListRequestsAsync` with a `ListRequestsFilter` (`Status`, `Type`, `MakerId`, `Search`, `Page`, `PerPage`), `RejectAsync`, `CancelAsync` and `AuditAsync`. Calls other than the event stream time out after `QuorumClientOptions.Timeout` (30 seconds by default).

## Payloads

`CreateRequest.Payload` is any object. It is serialised with `QuorumClientOptions.PayloadSerializerOptions`, which defaults to the web defaults (camelCase); set a snake_case policy if your policies' identity fields and bypass paths are written that way. `Request.PayloadAs<T>()` decodes it again.

The API expresses durations as Go strings such as `24h`. `Duration.Format(TimeSpan)` and `Duration.Parse(string)` convert.

## Errors

Every non-2xx response throws a `QuorumException` carrying `StatusCode`, the server's message as `Detail`, and the method and path that failed. `IsNotFound`, `IsConflict`, `IsForbidden`, `IsUnauthorized` and `IsBadRequest` cover the usual branches. A `403` means a [permission guard](/docs/concepts/permission-guards/) refused the decision; a `409` means the request is no longer pending, the checker already voted on this stage, or a pending request with the same identity fields exists.

## In this section

- [ASP.NET Core](/docs/clients/dotnet/aspnet-core/): `AddQuorumClient` and acting as the current user.
- [MediatR](/docs/clients/dotnet/mediatr/): approve commands with an attribute and no integration code.
- [Identity and authentication](/docs/clients/dotnet/authentication/): `Identity` against a `trust` server, `BearerToken` against a `verify` one, and renamed or attribute headers.
- [Decisions: streams, waiting and webhooks](/docs/clients/dotnet/decisions/): `EventsAsync`, `WaitForDecisionAsync` and `Webhooks.Parse`.
- [Policies and webhook subscriptions](/docs/clients/dotnet/policies/): the administrative surface.

## Where to look next

- The [package README](https://github.com/lawale/quorum/tree/master/clients/dotnet) covers development and how the tests run against the real server.
- `HealthAsync()` reports the server's release version and `QuorumClient.Version` the client's own, so you can confirm which Quorum and which client a deployment is running.
