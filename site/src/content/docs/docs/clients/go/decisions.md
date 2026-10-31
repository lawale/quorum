---
title: "Decisions: streams, waiting and webhooks"
description: Learning the outcome of a request from Go, by following its event stream, blocking until it is decided, or verifying the webhook Quorum delivers.
---

A request is decided by people, on their schedule. The client gives you three ways to find out, from the most passive to the most active.

## Webhooks

The usual way. Subscribe once (see [policies and webhook subscriptions](/docs/clients/go/policies/)) and Quorum delivers `approved`, `rejected`, `cancelled`, `expired` and `review_requested` events to your endpoint, signed with the subscription's secret. `ParseWebhook` checks the signature and decodes the body in one call:

```go
http.HandleFunc("/webhooks/quorum", func(w http.ResponseWriter, r *http.Request) {
    delivery, err := client.ParseWebhook(r, webhookSecret)
    if errors.Is(err, client.ErrInvalidSignature) {
        http.Error(w, "not from Quorum", http.StatusUnauthorized)
        return
    }
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    switch delivery.Event {
    case client.EventApproved:
        execute(delivery.Request)
    case client.EventReviewRequested:
        notify(delivery.Review)
    }
    w.WriteHeader(http.StatusNoContent)
})
```

`ParseWebhook` reads at most `MaxWebhookBody` bytes. Respond 2xx only once you have durably recorded the event, because Quorum retries anything else. Deliveries are at least once, so key your side effects on `delivery.Request.ID`.

A `WebhookPayload` carries the `Event` name, the full `Request`, the `Approvals` so far, the `Review` block on a `review_requested` event (who may act next, with scope already resolved), and a `Timestamp`. The [webhooks page](/docs/integrate/webhooks/) describes each event.

For bodies you have already read, `ParseWebhookBytes(body, signature, secret)` does the same work. `VerifySignature` checks without decoding, and `Sign` produces the header value, which is useful in tests and in proxies that re-sign deliveries.

## The event stream

`Events` subscribes to a request's Server-Sent Events and yields them as they arrive:

```go
for ev, err := range q.As(alice).Events(ctx, req.ID) {
    if err != nil {
        return err
    }
    if ev.Terminal() {
        break
    }
    latest, _ := q.As(alice).GetRequest(ctx, req.ID)
}
```

An event with an empty `Status` is an "updated" signal: something changed and the request should be re-read. `Status` is set only on the closing event of a request that is already terminal. The sequence ends when the server closes the stream, when `ctx` is done, or on a connection error, which is yielded last.

## Waiting for a decision

`WaitForDecision` wraps the stream in the common case: block until the request is terminal and return it.

```go
ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
defer cancel()

final, err := q.As(alice).WaitForDecision(ctx, req.ID, client.WaitOptions{PollInterval: 2 * time.Second})
if final.Status == client.StatusApproved { /* proceed */ }
```

It follows the event stream and falls back to polling `GetRequest` when streaming is unavailable or drops. `PollInterval` defaults to five seconds. Bound the wait with the context; a request that nobody decides on stays pending until its policy's expiry.
