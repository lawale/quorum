---
title: Identity and authentication
description: How the .NET client identifies the maker or checker on each call, against a server in trust mode or verify mode.
---

`As()` returns a client that acts as the given credentials. The client it was called on keeps its own, so a single `QuorumClient` registered at start-up serves every maker and checker in the process. Credentials are one of two records, matching the server's two [authentication modes](/docs/operate/authentication/).

## `Identity` for a server in `trust` mode

An `Identity` is sent as the gateway headers the server reads (`X-User-ID`, `X-User-Roles`, `X-User-Permissions`, `X-Tenant-ID` by default). Only use it from code that is the trusted gateway or sits behind one, since anyone who can set these headers can be anyone.

```csharp
var quorum = new QuorumClient(new Uri("https://quorum.example.com"), o => o.Tenant = "banking");

var maker = quorum.As(new Identity { UserId = "alice" });
var checker = quorum.As(new Identity
{
    UserId = "bob",
    Roles = ["manager"],
    Permissions = ["approve_transfer"],
});
```

`TenantId` on the identity overrides the client's `Tenant` option. Roles and permissions are joined with commas into their headers.

### Attributes

Identity attributes such as `branch_id` feed [reviewer scoping](/docs/concepts/reviewer-scoping/) and [bypass rules](/docs/concepts/bypass-rules/). The server reads each one from a header named in its `auth.trust.attribute_headers` configuration, and the client needs the same mapping to know where to send them:

```csharp
var quorum = new QuorumClient(new Uri("https://quorum.example.com"), o =>
{
    o.Tenant = "banking";
    o.TrustHeaders = new TrustHeaderNames
    {
        Attributes = new Dictionary<string, string> { ["branch_id"] = "X-User-Branch" },
    };
});

var teller = quorum.As(new Identity
{
    UserId = "alice",
    Attributes = new Dictionary<string, string> { ["branch_id"] = "ABJ-02" },
});
```

An attribute with no mapped header is not sent. `TrustHeaderNames` also renames the four standard headers; properties left at their defaults keep the server's default names.

## `BearerToken` for a server in `verify` mode

When the server validates JWTs, pass the user's token. It goes in the `Authorization` header and the server derives user, roles and tenant from the claims.

```csharp
var me = quorum.As(new BearerToken(accessToken));
var req = await me.CreateRequestAsync(new CreateRequest { Type = "wire_transfer", Payload = transfer });
```

## A default identity

`QuorumClientOptions.Credentials` sets the identity used by calls that are not made through `As()`, which suits a service account that manages policies and webhooks:

```csharp
builder.Services.AddQuorumClient(o =>
{
    o.BaseAddress = new Uri("https://quorum.example.com");
    o.Tenant = "banking";
    o.Credentials = new Identity { UserId = "deployer" };
});
```

A call with no identity at all is refused by the server with `401`.
