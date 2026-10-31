---
title: TypeScript client
description: The official TypeScript client for Quorum, for Node and the browser, with types generated from the OpenAPI document, per-call identity, webhook verification and waiting on a decision.
---

[`@oluwavader/quorum-client`](https://www.npmjs.com/package/@oluwavader/quorum-client) is the TypeScript client. It runs in Node 20+ and modern browsers, using `fetch`, Web Crypto and `ReadableStream` and nothing platform-specific.

```bash
npm install @oluwavader/quorum-client
```

Its request and response types are generated from the [OpenAPI document](/docs/integrate/api/), which the server is validated against in its test suite, so the client cannot drift from the API. On top of that it adds what REST cannot express: per-call identity, idempotent creation, webhook signature verification, an event stream, and waiting on a decision.

## Usage

A `QuorumClient` talks to one Quorum server. `as()` returns a client that acts as the given identity and leaves the original unchanged, so one instance serves every maker and checker in a process.

```ts
import { QuorumClient, isForbidden } from "@oluwavader/quorum-client";

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

// Resolve once the request is decided.
const final = await quorum.as({ userId: "alice" }).waitForDecision(req.id, {
  signal: AbortSignal.timeout(60_000),
});
```

The rest of the request surface follows the API one to one: `getRequest`, `listRequests` with a filter (`status`, `type`, `makerId`, `search`, `page`, `perPage`), `reject`, `cancel` and `audit`. Every call accepts an `AbortSignal`; calls other than the event stream time out after `timeoutMs` (30 seconds by default).

## Errors

Every non-2xx response throws a `QuorumError` carrying `status`, the server's message as `detail`, and the method and path that failed. `isNotFound`, `isConflict`, `isForbidden`, `isUnauthorized` and `isBadRequest` narrow an unknown error in one call. A `403` means a [permission guard](/docs/concepts/permission-guards/) refused the decision; a `409` means the request is no longer pending, the checker already voted on this stage, or a pending request with the same identity fields exists.

## In this section

- [Identity and authentication](/docs/clients/typescript/authentication/): an `Identity` against a `trust` server, `bearer()` against a `verify` one, and what is safe from a browser.
- [Decisions: streams, waiting and webhooks](/docs/clients/typescript/decisions/): `events()`, `waitForDecision()` and `parseWebhook()`.
- [Policies and webhook subscriptions](/docs/clients/typescript/policies/): the administrative surface.

## Where to look next

- The [package README](https://github.com/lawale/quorum/tree/master/clients/typescript) covers development and how the tests run against the real server.
- `health()` reports the server's release version and `VERSION` the client's own, so you can confirm which Quorum and which client a deployment is running.
- For a browser-only integration that needs approval screens rather than API calls, the [widgets](/docs/integrate/widgets/) ship their own small client.
