---
title: TypeScript client
description: The official TypeScript client for Quorum, for Node and the browser, with types generated from the OpenAPI document, per-call identity, webhook verification and waiting on a decision.
sidebar:
  order: 3
---

[`@oluwavader/quorum-client`](https://www.npmjs.com/package/@oluwavader/quorum-client) is the TypeScript client. It runs in Node 20+ and modern browsers, using `fetch`, Web Crypto and `ReadableStream` and nothing platform-specific.

```bash
npm install @oluwavader/quorum-client
```

Its request and response types are generated from the [OpenAPI document](/docs/integrate/api/), which the server is validated against in its test suite, so the client cannot drift from the API. On top of that it adds what REST cannot express: per-call identity, idempotent creation, webhook signature verification, an event stream, and waiting on a decision.

## Usage

A `QuorumClient` talks to one Quorum server. `as()` returns a client that acts as the given identity and leaves the original unchanged, so one instance serves every maker and checker in a process.

```ts
import { QuorumClient, parseWebhook, isForbidden, InvalidSignatureError } from "@oluwavader/quorum-client";

const quorum = new QuorumClient("https://quorum.example.com", { tenant: "banking" });

// The transfer's creator is the maker. A repeated idempotency key returns
// the same request instead of opening a second approval.
const req = await quorum.as({ userId: "alice" }).createRequest({
  type: "wire_transfer",
  payload: transfer,
  idempotencyKey: `transfer-${transfer.id}`,
});

// A checker decides.
try {
  await quorum.as({ userId: "bob", roles: ["manager"] }).approve(req.id, "looks right");
} catch (err) {
  if (isForbidden(err)) { /* bob may not act on this request */ }
}

// Resolve once the request is decided: follows the event stream, polls if
// streaming is unavailable, gives up when the signal does.
const final = await quorum.as({ userId: "alice" }).waitForDecision(req.id, {
  signal: AbortSignal.timeout(60_000),
});

// Verify and decode a webhook delivery in your handler. Pass the raw body.
try {
  const payload = await parseWebhook(rawBody, request.headers.get("X-Signature-256"), webhookSecret);
} catch (err) {
  if (err instanceof InvalidSignatureError) { /* not from Quorum */ }
}
```

Every non-2xx response throws a `QuorumError` carrying the status, the server's message and the call that failed. The `isNotFound`, `isConflict`, `isForbidden`, `isUnauthorized` and `isBadRequest` helpers narrow an unknown error in one call.

## Authentication

Against a server in `verify` mode, pass `bearer(jwt)` to `as()`. Against one in `trust` mode, pass an `Identity` (`userId`, `roles`, `permissions`, `tenantId`, `attributes`); it is sent as gateway headers, so only do this from code that is the trusted gateway or sits behind one. If the server reads identity from renamed headers, or attribute headers such as `X-User-Branch`, set them with the `trustHeaders` option. See [Authentication modes](/docs/operate/authentication/).

## Events and webhooks

`events(id)` is an async iterator over the request's Server-Sent Events. It ends when the request is terminal; an `updated` event means the request changed and should be re-read. `waitForDecision` is built on it.

`parseWebhook(body, signature, secret)` verifies the `X-Signature-256` header with a constant-time HMAC-SHA256 check and decodes the payload. Give it the body exactly as received: re-serialised JSON will not verify. See [Webhooks](/docs/integrate/webhooks/).

## Where to look next

- The [package README](https://github.com/lawale/quorum/tree/master/clients/typescript) covers the full surface, including policies and webhooks, and how the tests run against the real server.
- The client reports the server's release version from `health()`, and its own as `VERSION`, so you can confirm which Quorum and which client a deployment is running.
- For a browser-only integration that needs approval screens rather than API calls, the [widgets](/docs/integrate/widgets/) ship their own small client.
