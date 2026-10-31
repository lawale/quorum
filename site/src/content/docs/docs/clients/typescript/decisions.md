---
title: "Decisions: streams, waiting and webhooks"
description: Learning the outcome of a request from TypeScript, by following its event stream, awaiting the decision, or verifying the webhook Quorum delivers.
---

A request is decided by people, on their schedule. The client gives you three ways to find out, from the most passive to the most active.

## Webhooks

The usual way. Subscribe once (see [policies and webhook subscriptions](/docs/clients/typescript/policies/)) and Quorum delivers `approved`, `rejected`, `cancelled`, `expired` and `review_requested` events to your endpoint, signed with the subscription's secret. `parseWebhook` checks the signature and decodes the body in one call. It needs the body exactly as received, so read it raw rather than through a JSON body parser:

```ts
import express from "express";
import { parseWebhook, InvalidSignatureError, SIGNATURE_HEADER } from "@oluwavader/quorum-client";

app.post("/webhooks/quorum", express.raw({ type: "application/json" }), async (req, res) => {
  let delivery;
  try {
    delivery = await parseWebhook(req.body, req.header(SIGNATURE_HEADER), webhookSecret);
  } catch (err) {
    if (err instanceof InvalidSignatureError) return res.status(401).end();
    return res.status(400).end();
  }
  if (delivery.event === "approved") await execute(delivery.request);
  if (delivery.event === "review_requested") await notify(delivery.review);
  res.status(204).end();
});
```

Respond 2xx only once you have durably recorded the event, because Quorum retries anything else. Deliveries are at least once, so key your side effects on `delivery.request.id`.

A `WebhookPayload` carries the `event` name, the full `request`, the `approvals` so far, the `review` block on a `review_requested` event (who may act next, with scope already resolved), and a `timestamp`. The [webhooks page](/docs/integrate/webhooks/) describes each event.

`verifySignature(body, signature, secret)` checks without decoding, and `sign(body, secret)` produces the header value, which is useful in tests and in proxies that re-sign deliveries. The comparison is constant-time.

## The event stream

`events()` is an async iterator over a request's Server-Sent Events:

```ts
import { isTerminalEvent } from "@oluwavader/quorum-client";

for await (const ev of quorum.as(alice).events(req.id, { signal })) {
  if (isTerminalEvent(ev)) break;
  const latest = await quorum.as(alice).getRequest(req.id);
}
```

An event with no `status` is an "updated" signal: something changed and the request should be re-read. `status` is set only on the closing event of a request that is already terminal. The iterator ends when the server closes the stream or the signal aborts.

## Waiting for a decision

`waitForDecision()` wraps the stream in the common case: resolve once the request is terminal.

```ts
const final = await quorum.as(alice).waitForDecision(req.id, {
  signal: AbortSignal.timeout(10 * 60_000),
  pollIntervalMs: 2_000,
});
if (final.status === "approved") { /* proceed */ }
```

It follows the event stream and falls back to polling `getRequest()` when streaming is unavailable or drops. `pollIntervalMs` defaults to five seconds. Always pass a signal; a request that nobody decides on stays pending until its policy's expiry.
