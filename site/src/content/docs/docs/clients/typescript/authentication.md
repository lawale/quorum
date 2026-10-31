---
title: Identity and authentication
description: How the TypeScript client identifies the maker or checker on each call, against a server in trust mode or verify mode, and what is safe from a browser.
---

`as()` returns a new client that acts as the given credentials; the original is unchanged, so one `QuorumClient` built at start-up serves every maker and checker in the process. Credentials are one of two shapes, matching the server's two [authentication modes](/docs/operate/authentication/).

## An `Identity` for a server in `trust` mode

An `Identity` is sent as the gateway headers the server reads (`X-User-ID`, `X-User-Roles`, `X-User-Permissions`, `X-Tenant-ID` by default). Only use it from code that is the trusted gateway or sits behind one, since anyone who can set these headers can be anyone. In practice that means a server-side process, never a browser.

```ts
const quorum = new QuorumClient("https://quorum.example.com", { tenant: "banking" });

const maker = quorum.as({ userId: "alice" });
const checker = quorum.as({
  userId: "bob",
  roles: ["manager"],
  permissions: ["approve_transfer"],
});
```

`tenantId` on the identity overrides the client's `tenant` option. Roles and permissions are joined with commas into their headers.

### Attributes

Identity attributes such as `branch_id` feed [reviewer scoping](/docs/concepts/reviewer-scoping/) and [bypass rules](/docs/concepts/bypass-rules/). The server reads each one from a header named in its `auth.trust.attribute_headers` configuration, and the client needs the same mapping to know where to send them:

```ts
const quorum = new QuorumClient("https://quorum.example.com", {
  tenant: "banking",
  trustHeaders: { attributes: { branch_id: "X-User-Branch" } },
});

const teller = quorum.as({ userId: "alice", attributes: { branch_id: "ABJ-02" } });
```

An attribute with no mapped header is not sent. `trustHeaders` also renames the four standard headers; fields left out keep the values in `DEFAULT_HEADER_NAMES`.

## `bearer()` for a server in `verify` mode

When the server validates JWTs, pass the user's token. It goes in the `Authorization` header and the server derives user, roles and tenant from the claims. This is the shape to use in a browser, where the client acts as the signed-in user and nothing else.

```ts
import { bearer } from "@oluwavader/quorum-client";

const me = quorum.as(bearer(session.accessToken));
const req = await me.createRequest({ type: "wire_transfer", payload: transfer });
```

## A default identity

The `credentials` option sets the identity used by calls that are not made through `as()`, which suits a service account that manages policies and webhooks:

```ts
const admin = new QuorumClient("https://quorum.example.com", {
  tenant: "banking",
  credentials: { userId: "deployer" },
});
```

A call with no identity at all is refused by the server with `401`.
