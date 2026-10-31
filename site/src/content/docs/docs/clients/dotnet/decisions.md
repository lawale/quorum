---
title: "Decisions: streams, waiting and webhooks"
description: Learning the outcome of a request from .NET, by following its event stream, awaiting the decision, or verifying the webhook Quorum delivers.
---

A request is decided by people, on their schedule. The client gives you three ways to find out, from the most passive to the most active.

## Webhooks

The usual way. Subscribe once (see [policies and webhook subscriptions](/docs/clients/dotnet/policies/)) and Quorum delivers `approved`, `rejected`, `cancelled`, `expired` and `review_requested` events to your endpoint, signed with the subscription's secret. `Webhooks.ParseAsync` checks the signature and decodes the body in one call. Give it the request body stream, not a model bound by the framework, because re-serialised JSON will not verify:

```csharp
app.MapPost("/webhooks/quorum", async (HttpRequest http, CancellationToken ct) =>
{
    WebhookPayload delivery;
    try
    {
        delivery = await Webhooks.ParseAsync(http.Body, http.Headers[Webhooks.SignatureHeader], webhookSecret, ct);
    }
    catch (InvalidSignatureException) { return Results.Unauthorized(); }

    switch (delivery.Event)
    {
        case WebhookEvent.Approved: await Execute(delivery.Request); break;
        case WebhookEvent.ReviewRequested: await Notify(delivery.Review!); break;
    }
    return Results.NoContent();
});
```

`ParseAsync` reads at most `Webhooks.MaxBodyBytes`. Respond 2xx only once you have durably recorded the event, because Quorum retries anything else. Deliveries are at least once, so key your side effects on `delivery.Request.Id`.

A `WebhookPayload` carries the `Event`, the full `Request`, the `Approvals` so far, the `Review` block on a `review_requested` event (who may act next, with scope already resolved), and a `Timestamp`. The [webhooks page](/docs/integrate/webhooks/) describes each event.

For bodies you already hold as a string or bytes, `Webhooks.Parse` does the same work synchronously. `Webhooks.Verify` checks without decoding, and `Webhooks.Sign` produces the header value, which is useful in tests and in proxies that re-sign deliveries. The comparison is constant-time.

Applications on MediatR can skip the endpoint body entirely: the [MediatR package](/docs/clients/dotnet/mediatr/) verifies the delivery and dispatches the approved command.

## The event stream

`EventsAsync` is an `IAsyncEnumerable<StreamEvent>` over a request's Server-Sent Events:

```csharp
await foreach (var ev in quorum.As(alice).EventsAsync(req.Id, ct))
{
    if (ev.IsTerminal) break;
    var latest = await quorum.As(alice).GetRequestAsync(req.Id, ct);
}
```

An event with a null `Status` is an "updated" signal: something changed and the request should be re-read. `Status` is set only on the closing event of a request that is already terminal. The enumeration ends when the server closes the stream or the token is cancelled. Streams are not subject to the per-call timeout.

## Waiting for a decision

`WaitForDecisionAsync` wraps the stream in the common case: complete once the request is terminal.

```csharp
using var cts = new CancellationTokenSource(TimeSpan.FromMinutes(10));
var final = await quorum.As(alice).WaitForDecisionAsync(req.Id, pollInterval: TimeSpan.FromSeconds(2), cancellationToken: cts.Token);
if (final.Status == RequestStatus.Approved) { /* proceed */ }
```

It follows the event stream and falls back to polling `GetRequestAsync` when streaming is unavailable or drops. `pollInterval` defaults to five seconds. Always pass a cancellation token; a request that nobody decides on stays pending until its policy's expiry.
