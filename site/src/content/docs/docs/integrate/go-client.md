---
title: Go client
description: The official Go client for Quorum, with per-call identity, idempotent creation, webhook signature verification and waiting on a decision.
sidebar:
  order: 2
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

// Block until the request is decided: follows the event stream, polls if
// streaming is unavailable, gives up when ctx does.
final, err := q.As(client.Identity{UserID: "alice"}).WaitForDecision(ctx, req.ID)

// Verify and decode a webhook delivery in your handler.
payload, err := client.ParseWebhook(r, webhookSecret)
if errors.Is(err, client.ErrInvalidSignature) { /* not from Quorum */ }
```

## Authentication

Against a server in `verify` mode, pass `client.BearerToken(jwt)` to `As` instead of an `Identity`. Against one in `trust` mode with renamed headers, set them with `client.WithTrustHeaderNames`. See [Authentication modes](/docs/operate/authentication/).

## Where to look next

- The package documentation on [pkg.go.dev](https://pkg.go.dev/github.com/lawale/quorum/clients/go) lists every method, including policies and webhooks.
- The [banking sample app](/docs/demo/) is built on the client: it creates transfers, verifies the approved webhook with `ParseWebhook` and executes the transfer.
- The client reports the server's release version, so you can confirm which Quorum you are talking to.
