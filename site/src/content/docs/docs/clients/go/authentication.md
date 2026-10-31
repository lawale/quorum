---
title: Identity and authentication
description: How the Go client identifies the maker or checker on each call, against a server in trust mode or verify mode.
---

`As` returns a client that acts as the given credentials. The client it was called on keeps its own, so a single `Client` built at start-up can serve every maker and checker in the process. Credentials are one of two types, matching the server's two [authentication modes](/docs/operate/authentication/).

## `Identity` for a server in `trust` mode

An `Identity` is sent as the gateway headers the server reads (`X-User-ID`, `X-User-Roles`, `X-User-Permissions`, `X-Tenant-ID` by default). Only use it from code that is the trusted gateway or sits behind one, since anyone who can set these headers can be anyone.

```go
q, err := client.New("https://quorum.example.com", client.WithTenant("banking"))

maker := q.As(client.Identity{UserID: "alice"})
checker := q.As(client.Identity{
    UserID:      "bob",
    Roles:       []string{"manager"},
    Permissions: []string{"approve_transfer"},
})
```

`TenantID` on the identity overrides the client's `WithTenant`. Roles and permissions are joined with commas into their headers.

### Attributes

Identity attributes such as `branch_id` feed [reviewer scoping](/docs/concepts/reviewer-scoping/) and [bypass rules](/docs/concepts/bypass-rules/). The server reads each one from a header named in its `auth.trust.attribute_headers` configuration, and the client needs the same mapping to know where to send them:

```go
q, err := client.New("https://quorum.example.com",
    client.WithTenant("banking"),
    client.WithTrustHeaderNames(client.HeaderNames{
        Attributes: map[string]string{"branch_id": "X-User-Branch"},
    }),
)

teller := q.As(client.Identity{
    UserID:     "alice",
    Attributes: map[string]string{"branch_id": "ABJ-02"},
})
```

An attribute with no mapped header is not sent. `HeaderNames` also renames the four standard headers; fields left empty keep the defaults from `DefaultHeaderNames`.

## `BearerToken` for a server in `verify` mode

When the server validates JWTs, pass the user's token. It goes in the `Authorization` header and the server derives user, roles and tenant from the claims.

```go
me := q.As(client.BearerToken(accessToken))
req, err := me.CreateRequest(ctx, client.CreateRequest{Type: "wire_transfer", Payload: transfer})
```

## A default identity

`WithCredentials` sets the identity used by calls that are not made through `As`, which suits a service account that manages policies and webhooks:

```go
admin, err := client.New("https://quorum.example.com",
    client.WithTenant("banking"),
    client.WithCredentials(client.Identity{UserID: "deployer"}),
)
```

A call with no identity at all is refused by the server with `401`.
