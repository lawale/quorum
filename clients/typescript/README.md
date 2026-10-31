# @oluwavader/quorum-client

TypeScript client for [Quorum](https://github.com/lawale/quorum), the approval engine. Runs in Node 20+ and modern browsers; it uses `fetch`, Web Crypto and `ReadableStream`, nothing platform-specific.

```bash
npm install @oluwavader/quorum-client
```

The request and response types are generated from Quorum's OpenAPI document, which the server is validated against in its test suite, so the client cannot drift from the API. On top of that it adds what REST cannot express: per-call identity, idempotent creation, webhook signature verification, an event stream, and waiting on a decision.

## Usage

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
  if (isForbidden(err)) {
    /* bob may not act on this request */
  }
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
  if (err instanceof InvalidSignatureError) {
    /* not from Quorum */
  }
}
```

### Identity

`as()` returns a new client that acts as the given credentials; the original is unchanged, so one `QuorumClient` serves every maker and checker in a process.

- Against a server in `trust` mode, pass an `Identity`: `{ userId, roles, permissions, tenantId, attributes }`. It is sent as gateway headers, so only do this from code that *is* the trusted gateway or sits behind one. If the server reads identity from renamed headers, or from attribute headers such as `X-User-Branch`, set them with the `trustHeaders` option.
- Against a server in `verify` mode, pass `bearer(jwt)`.

### Errors

Every non-2xx response throws a `QuorumError` carrying `status`, the server's message as `detail`, and the method and path. The `isNotFound`, `isConflict`, `isForbidden`, `isUnauthorized` and `isBadRequest` helpers narrow an unknown error in one call.

### Events

`events(id)` is an async iterator over the request's server-sent events. It ends when the request is terminal; an `updated` event means the request changed and should be re-read. `waitForDecision` builds on it.

### Webhooks

`parseWebhook(body, signature, secret)` verifies the `X-Signature-256` header (HMAC-SHA256, constant-time) and decodes the payload. Give it the body exactly as received; re-serialised JSON will not verify. `sign` produces the header value for tests and proxies.

## Development

```bash
npm run generate   # regenerate src/generated/schema.ts from ../../api/openapi.yaml
npm run typecheck
npm test           # builds and runs the Go test server, then the suite against it
npm run build
```

The tests need Go on the path. They run against the real Quorum server over in-memory stores, with every request and response validated against the OpenAPI document; the run fails if any traffic disagrees with the document or any documented operation is never called.

The package version is set from the git tag at release time, and `VERSION` reports it at runtime. A checkout reports `0.0.0`.
