# Quorum.Client

.NET client for [Quorum](https://github.com/lawale/quorum), the approval engine. Targets .NET 8 and .NET 10, depends only on the BCL and `Microsoft.Extensions.Http`.

```bash
dotnet add package Quorum.Client
```

The models are records that mirror Quorum's OpenAPI document, and the test suite drives the real server with unmapped members disallowed, so a field the server sends that the client lacks fails the build rather than vanishing. On top of the API the client adds what REST cannot express: per-call identity, idempotent creation, webhook signature verification, an event stream, and waiting on a decision.

## Usage

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
catch (QuorumException e) when (e.IsForbidden)
{
    // bob may not act on this request
}

using var cts = new CancellationTokenSource(TimeSpan.FromMinutes(1));
var final = await quorum.As(new Identity { UserId = "alice" }).WaitForDecisionAsync(req.Id, cancellationToken: cts.Token);

// Pass the raw body: re-serialised JSON will not verify.
try
{
    var payload = await Webhooks.ParseAsync(httpRequest.Body, httpRequest.Headers[Webhooks.SignatureHeader], webhookSecret);
}
catch (InvalidSignatureException)
{
    // not from Quorum
}
```

### ASP.NET Core

```csharp
builder.Services.AddQuorumClient(o =>
{
    o.BaseAddress = new Uri(builder.Configuration["Quorum:Url"]!);
    o.Tenant = "banking";
});
```

Inject `QuorumClient` and call `As(...)` with the current user's identity per request. The registration goes through `IHttpClientFactory` with an unbounded `HttpClient` timeout so event streams are not cut off; the per-call `QuorumClientOptions.Timeout` (30 seconds by default) still applies.

### MediatR

[`Quorum.Client.MediatR`](src/Quorum.Client.MediatR) removes the integration code entirely for applications built on MediatR. Mark a command with `[RequiresApproval]`, and `Send` opens an approval request instead of running the handler; when a checker approves, the same command is dispatched to the same handler from the webhook. Bypass rules run the handler inline. See its [README](src/Quorum.Client.MediatR/README.md).

### Identity

`As()` returns a new client acting as the given credentials; the original is unchanged, so one `QuorumClient` serves every maker and checker in a process.

- Against a server in `trust` mode, pass an `Identity` (`UserId`, `Roles`, `Permissions`, `TenantId`, `Attributes`). It is sent as gateway headers, so only do this from code that *is* the trusted gateway or sits behind one. If the server reads identity from renamed headers, or from attribute headers such as `X-User-Branch`, set `TrustHeaders`.
- Against a server in `verify` mode, pass `new BearerToken(jwt)`.

### Payloads

`CreateRequest.Payload` is any object. It is serialised with `QuorumClientOptions.PayloadSerializerOptions`, which defaults to the web defaults (camelCase); set a snake_case policy if your policies' identity fields and bypass paths are written that way. `Request.PayloadAs<T>()` decodes it again.

### Errors

Every non-2xx response throws a `QuorumException` carrying `StatusCode`, the server's message as `Detail`, and the call that failed. `IsNotFound`, `IsConflict`, `IsForbidden`, `IsUnauthorized` and `IsBadRequest` cover the usual branches.

### Durations

The API expresses durations as Go strings such as `24h`. `Duration.Format(TimeSpan)` and `Duration.Parse(string)` convert.

## Development

```bash
dotnet build
dotnet test        # builds and runs the Go test server, then the suite against it
dotnet pack -p:Version=1.2.3
```

The tests need Go on the path. They run against the real Quorum server over in-memory stores, with every request and response validated against the OpenAPI document; the run fails if any traffic disagrees with the document, if the server sends a field the models do not declare, or if any documented operation is never called.

The package version is set from the git tag at release time, and `QuorumClient.Version` reports it at runtime. A checkout reports `0.0.0`.
