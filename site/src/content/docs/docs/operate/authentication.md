---
title: Authentication modes
description: How Quorum identifies callers, by validating JWTs against your identity provider's JWKS endpoint or by trusting headers set by your gateway.
sidebar:
  order: 2
---

Authentication is selected via `auth.mode`. Both implemented modes produce the same identity (user ID, roles, permissions, tenant, and any configured attributes), so policies and the rest of the system do not care which one is in use.

## `verify`: JWT validation against your identity provider

Quorum validates the `Authorization: Bearer <jwt>` header on every request against the signing keys published at your provider's JWKS URL. This is the mode to use when browsers or third-party services call Quorum directly, including the [embeddable widgets](/docs/integrate/widgets/). It works with Keycloak, Auth0, Entra ID, Cognito, or anything OIDC-shaped.

```yaml
auth:
  mode: verify
  verify:
    jwks_url: https://login.example.com/realms/acme/protocol/openid-connect/certs
    issuer: https://login.example.com/realms/acme
    audience: quorum
    claims:
      user_id: sub
      roles: realm_access.roles   # Keycloak nests roles; dotted paths are supported
      tenant_id: tenant_id        # custom claim added by a mapper
```

What is enforced:

- **Signature** against a key from the JWKS, matched by `kid`. Only asymmetric algorithms (RS*, PS*, ES*) are accepted; HMAC and `none` are rejected regardless of the token header.
- **Expiry** (`exp`, required), `nbf`, and `iat`, with `clock_skew` leeway.
- **Issuer** and **audience**, when configured.
- **Identity claims**: the user ID claim must be present. The tenant claim must be present unless `default_tenant_id` is set.

Key rotation is automatic: keys are refreshed in the background every `jwks_refresh`, and a token signed with an unknown `kid` triggers an immediate refresh (rate-limited to once a minute, so a flood of bad tokens cannot be used to hammer your provider). If a refresh fails the previously cached keys stay in use, and if the provider is unreachable at startup Quorum still starts and fetches keys on the first request.

Roles and permissions accept the shapes providers actually emit: a JSON array of strings, or a single space- or comma-separated string such as an OAuth `scope` claim.

Auth0 example, where roles usually arrive under a namespaced claim and permissions in `permissions`:

```yaml
auth:
  mode: verify
  verify:
    jwks_url: https://acme.us.auth0.com/.well-known/jwks.json
    issuer: https://acme.us.auth0.com/
    audience: https://quorum.acme.internal
    claims:
      roles: https://acme.com/roles
      permissions: permissions
      tenant_id: https://acme.com/tenant
```

## `trust`: identity from gateway headers

Reads user identity from request headers (`X-User-ID`, `X-User-Roles`, `X-User-Permissions`, `X-Tenant-ID`; names are configurable). Suitable when Quorum sits behind a gateway, service mesh, or backend-for-frontend that has already authenticated the user and sets these headers.

Quorum must not be reachable directly by end users in this mode, since anyone who can set the headers can impersonate anyone. The [sample apps](/docs/demo/) run this way: each app's backend knows the signed-in persona and sets the headers on its own calls.

## `custom` (planned)

Delegate authentication to an external HTTP endpoint, for identity systems that do not issue JWTs.

Note that stage-level [dynamic authorization checks](/docs/concepts/permission-guards/#dynamic-authorization-check) are a separate, implemented feature: once a user is authenticated, a policy can still call out to your endpoint to decide whether that user may act on a given stage.

## Identity attributes

Beyond roles and permissions, both modes can read named scalar attributes, such as a branch or department, from headers or claims. These feed [reviewer scoping](/docs/concepts/reviewer-scoping/) and [bypass rules](/docs/concepts/bypass-rules/), and are passed to the dynamic authorization URL.
