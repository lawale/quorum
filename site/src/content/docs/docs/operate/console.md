---
title: Admin console
description: The embedded admin console for managing policies and webhooks, browsing requests and audit logs, and watching deliveries. Opt-in at build time.
sidebar:
  order: 4
---

Quorum includes an optional embedded admin console, a web UI for managing policies, webhooks, viewing requests, and browsing audit logs. It is **opt-in** via a Go build tag. The standard binary has no front-end assets and no `/console` route.

## Build with the console

```bash
make build-console
```

Or use the `ghcr.io/lawale/quorum-console` or `ghcr.io/lawale/quorum-all` [images](/docs/getting-started/docker/).

## Enable in config

```yaml
console:
  enabled: true
  jwt_secret: ""  # optional; auto-generated if empty (sessions lost on restart)
  secure_cookies: true  # behind HTTPS
```

Visit `http://localhost:8080/console/` after starting the server. On first run, you will be prompted to create an initial admin operator. Subsequent operators can be added through the UI. Console operators are separate from the makers and checkers your identity provider knows about; they administer Quorum rather than act on requests.

## Role and permission suggestions

Optionally configure `roles_url` and `permissions_url` to point at your identity provider's API. The console proxies these URLs server-side and populates dropdown suggestions in the policy editor, while still supporting free-text entry. The endpoints must return a JSON array of strings, such as `["admin", "manager", "viewer"]`.

```yaml
console:
  enabled: true
  suggestions:
    roles_url: "https://your-idp.example.com/api/roles"
    permissions_url: "https://your-idp.example.com/api/permissions"
    auth_header: "X-Api-Key"
    auth_value: "your-idp-api-key"
```

## Features

- Dashboard overview of pending and decided requests.
- Policy CRUD with a multi-stage editor: role and permission inputs with external suggestions, authorization mode, scope rules, bypass rules, display templates and client-side validation. Warnings are shown when a scope rule references an attribute the active auth mode does not supply.
- Webhook management with delivery rate colour coding.
- Request browser with status and type filters and copy-to-clipboard IDs.
- Request detail with tabbed payload, resolved display fields or before/after changes, and the audit trail.
- Audit log with policy context.
- Delivery browser with partial event matching.
- Operator management.
