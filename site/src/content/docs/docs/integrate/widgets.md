---
title: Embeddable widgets
description: Three Lit Web Components, an approval panel, a request list and a stage progress bar, that give any front end approval UI with live updates over SSE.
sidebar:
  order: 5
---

Quorum provides three Web Components that you can embed directly in your application's UI. They connect to the Quorum API and render approval workflows, with no custom UI code needed. They are built with Lit, use Shadow DOM for style isolation, work with any front-end framework, and are themeable via CSS custom properties and `::part()`.

| Element | What it renders |
|---------|-----------------|
| `<quorum-approval-panel>` | One request's details, resolved display fields or before/after changes, and approve and reject actions when the viewer may act. |
| `<quorum-request-list>` | A filterable, paginated list of requests, suitable for an approval queue. |
| `<quorum-stage-progress>` | The stages of a request and where it currently sits, updated live. |

## Building the server with widgets

The widget bundle is served by the Quorum server itself and is opt-in via a Go build tag:

```bash
make build-embed   # widgets only
make build-all     # console and widgets
```

The bundle is served at `/assets/embed.js` with CORS headers, so you can load it from any origin. The `ghcr.io/lawale/quorum-all` image includes it.

## Loading the widgets

**Option 1: script tag**, loaded from your Quorum server:

```html
<script src="https://your-quorum-host/assets/embed.js"></script>
```

**Option 2: npm**, bundled into your own build:

```bash
npm install @oluwavader/quorum-embed
```

```typescript
// Registers all three custom elements
import '@oluwavader/quorum-embed';

// Optionally use the API client directly
import { createClient } from '@oluwavader/quorum-embed';

const client = createClient({
  apiUrl: 'https://your-quorum-host',
  token: session.accessToken, // sent as "Authorization: Bearer ..."; server in verify mode
});
const request = await client.getRequest('uuid-here');
```

## Configure once, then embed

```js
// With the script tag the API is on window.QuorumEmbed
QuorumEmbed.configure({
  apiUrl: 'https://your-quorum-host',
  token: () => session.accessToken, // static value or getter, called on every request
});
```

```html
<!-- Show approval details with approve/reject actions -->
<quorum-approval-panel request-id="uuid-here"></quorum-approval-panel>

<!-- List requests with filters and pagination -->
<quorum-request-list status="pending" page-size="10"></quorum-request-list>

<!-- Visualize approval stage progress (with real-time SSE updates) -->
<quorum-stage-progress request-id="uuid-here" poll-interval="30000"></quorum-stage-progress>
```

Any attribute (`api-url`, `token`, `tenant-id`, `auth-headers`) set on an element overrides the global value for that element.

## Authentication

With the server in `verify` mode, pass the user's access token via `token` (or a getter in `configure()`); it is sent as `Authorization: Bearer <token>` and the tenant comes from the token's claims.

With the server in `trust` mode, pass `auth-headers` (a JSON string) such as `{"X-User-ID": "alice", "X-User-Roles": "manager"}` instead, and make sure the page is served by a backend that already knows the user, as the sample apps do, with Quorum not exposed directly.

Approve and Reject buttons only render when the server reports the viewer may act on the request.

## Theming

Set CSS custom properties such as `--quorum-accent`, `--quorum-bg`, or `--quorum-font` on any ancestor to restyle every widget, and use `::part()` selectors for structural overrides. The full token and part list is in the [package README](https://github.com/lawale/quorum/blob/master/widgets/frontend/README.md#theming).

## Real-time updates

Both the approval panel and stage progress widgets connect via Server-Sent Events by default for instant push notifications. Set `sse="false"` to disable and use polling only. The `poll-interval` attribute (default 30000ms) controls the fallback polling rate. When both widgets are on the same page, the stage progress widget also reacts to custom events dispatched by the approval panel for immediate cross-widget updates.

## Events and errors

Widgets dispatch custom events you can listen for: `quorum:approved`, `quorum:rejected`, `quorum:select`, and `quorum:error`.

Set `suppress-errors` on a widget to hide inline error messages and handle `quorum:error` externally, for example with a toast. The event detail includes `{ action, message, status }` where `action` is `"load"`, `"approve"`, or `"reject"`.

## JavaScript API

The package also exports `configure(config)`, `createClient(config)` and the element classes, for use without the custom elements or from a framework wrapper. See the [package README](https://github.com/lawale/quorum/blob/master/widgets/frontend/README.md#javascript-api).
