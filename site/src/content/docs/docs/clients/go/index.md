---
title: Go client
description: The official Go client for Quorum. It shares the server's own types and adds per-call identity, idempotent creation, webhook verification and waiting on a decision.
---

`github.com/lawale/quorum/clients/go` is the Go client. It shares the server's own types, so it cannot drift from the API, and adds what a REST surface cannot express: per-call identity, idempotent creation, webhook signature verification, and waiting on a decision.

```bash
go get github.com/lawale/quorum/clients/go
```

## Usage

A `Client` talks to one Quorum server. Identity is attached per call with `As`, so one client serves every maker and checker in a process.

```go
import "github.com/lawale/quorum/clients/go"

q, err := client.New("https://quorum.example.com", client.WithTenant("banking"))

// The transfer's creator is the maker. A repeated idempotency key returns
// the same request instead of opening a second approval.
req, err := q.As(client.Identity{UserID: "alice"}).CreateRequest(ctx, client.CreateRequest{
    Type:           "wire_transfer",
    Payload:        transfer,
    IdempotencyKey: "transfer-" + transfer.ID,
})

// A checker decides.
req, err = q.As(client.Identity{UserID: "bob", Roles: []string{"manager"}}).Approve(ctx, req.ID, "looks right")
if client.IsForbidden(err) { /* bob may not act on this request */ }

// Block until the request is decided.
final, err := q.As(client.Identity{UserID: "alice"}).WaitForDecision(ctx, req.ID)
```

`Payload` is any value that marshals to a non-empty JSON object, or a `json.RawMessage` holding one. The request comes back as the server's `model.Request`, with `Status`, `CurrentStage`, `Approvals` and `ViewerCanAct` for the identity that made the call.

The rest of the request surface follows the API one to one: `GetRequest`, `ListRequests` with a `ListRequests` filter (`Status`, `Type`, `MakerID`, `Search`, `Page`, `PerPage`), `Reject`, `Cancel` and `Audit`.

## Errors

Every non-2xx response is a `*client.Error` carrying `StatusCode` and the server's `Message`. `IsNotFound`, `IsConflict`, `IsForbidden`, `IsUnauthorized` and `IsBadRequest` test it through `errors.As`, so they work on wrapped errors too. A `403` means a [permission guard](/docs/concepts/permission-guards/) refused the decision; a `409` means the request is no longer pending, the checker already voted on this stage, or a pending request with the same identity fields exists.

## In this section

- [Identity and authentication](/docs/clients/go/authentication/): `Identity` against a `trust` server, `BearerToken` against a `verify` one, and renamed or attribute headers.
- [Decisions: streams, waiting and webhooks](/docs/clients/go/decisions/): `Events`, `WaitForDecision` and `ParseWebhook`.
- [Policies and webhook subscriptions](/docs/clients/go/policies/): the administrative surface.

## Where to look next

- The package documentation on [pkg.go.dev](https://pkg.go.dev/github.com/lawale/quorum/clients/go) lists every method.
- The [banking sample app](/docs/demo/) is built on the client: it creates transfers, verifies the approved webhook with `ParseWebhook` and executes the transfer.
- `Health` reports the server's release version and `client.Version()` the client's own, so you can confirm which Quorum and which client a deployment is running.
