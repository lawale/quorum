---
title: ASP.NET Core
description: Registering the Quorum .NET client with the host's HttpClientFactory and acting as the signed-in user on each call.
---

`AddQuorumClient` registers one `QuorumClient` for the application, backed by a named `HttpClient` from `IHttpClientFactory`.

```csharp
builder.Services.AddQuorumClient(o =>
{
    o.BaseAddress = new Uri(builder.Configuration["Quorum:Url"]!);
    o.Tenant = "banking";
});
```

It returns the `IHttpClientBuilder`, so the usual handlers can be chained on: resilience policies, logging, a message handler that injects a service credential.

## Acting as the current user

Inject `QuorumClient` and call `As()` with the signed-in user's identity per request. The client is safe to share; `As()` produces a lightweight view with different credentials and leaves the registered instance unchanged.

```csharp
app.MapPost("/transfers", async (TransferForm form, QuorumClient quorum, HttpContext http) =>
{
    var me = quorum.As(new Identity
    {
        UserId = http.User.FindFirstValue(ClaimTypes.NameIdentifier)!,
        Roles = http.User.FindAll(ClaimTypes.Role).Select(c => c.Value).ToList(),
    });
    var request = await me.CreateRequestAsync(new CreateRequest { Type = "wire_transfer", Payload = form });
    return Results.Accepted($"/transfers/{request.Id}", request);
});
```

If Quorum runs in `verify` mode and your application holds the user's access token, pass `new BearerToken(token)` instead and let Quorum read the claims itself. See [identity and authentication](/docs/clients/dotnet/authentication/).

## Timeouts

The registration sets the `HttpClient` timeout to infinite so that event streams and `WaitForDecisionAsync` are not cut off mid-wait. Ordinary calls are still bounded by `QuorumClientOptions.Timeout`, 30 seconds by default.

## Options

| Option | Purpose |
|--------|---------|
| `BaseAddress` | The Quorum server. Required unless the `HttpClient` already has one. |
| `Tenant` | Sent with trusted identities that do not name a tenant themselves. |
| `Credentials` | Identity used by calls not made through `As()`. |
| `TrustHeaders` | Header names a `trust` mode server reads identity from, including attribute headers. |
| `Timeout` | Per-call timeout. Event streams are not bounded. |
| `PayloadSerializerOptions` | How request payloads and metadata are serialised. Web defaults unless changed. |
| `UserAgent` | Replaces the default `quorum-dotnet-client/<version>`. |

For applications built on MediatR, the [MediatR package](/docs/clients/dotnet/mediatr/) sits on top of this registration and removes the per-endpoint code entirely.
