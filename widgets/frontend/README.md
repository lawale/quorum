# @oluwavader/quorum-embed

Embeddable Web Components for the [Quorum](https://github.com/lawale/quorum) approval engine, built with [Lit](https://lit.dev/). Drop approval workflows into any web app. The host page needs no framework; the elements work in plain HTML, React, Vue, Svelte, Angular, or anything else that renders DOM.

## Install

**From npm**, bundled into your own build:

```bash
npm install @oluwavader/quorum-embed
```

```js
import '@oluwavader/quorum-embed'; // registers all three elements
```

**From your Quorum server**, if it was built with the `embed` tag. The bundle is served with CORS headers so any origin can load it:

```html
<script src="https://your-quorum-host/assets/embed.js"></script>
```

**From a CDN:**

```html
<script src="https://unpkg.com/@oluwavader/quorum-embed/dist/embed.js"></script>
```

The script-tag builds expose the JavaScript API on `window.QuorumEmbed`.

## Quick Setup

Set global defaults once, then drop elements anywhere:

```js
import { configure } from '@oluwavader/quorum-embed';

configure({
  apiUrl: 'https://your-quorum-server.com',
  tenantId: 'your-tenant-id',
  token: () => auth.getAccessToken(), // static value or getter, called on every request
});
```

```html
<quorum-approval-panel request-id="req_abc123"></quorum-approval-panel>
```

Per-element attributes override global config. Order does not matter: elements that mount before `configure()` runs load as soon as it is called.

## Authentication

Two mechanisms, matching the server's auth modes:

- **`token`** sends `Authorization: Bearer <token>`, for a server in `verify` mode. Pass the raw access token from your identity provider; the `Bearer ` prefix is added for you. Prefer a getter in `configure()` so refreshed tokens are picked up on every request. The tenant comes from the token's claims, so `tenant-id` is not needed.
- **`auth-headers`** sends arbitrary headers as a JSON string, for a server in `trust` mode, e.g. `{"X-User-ID": "alice", "X-User-Roles": "manager"}`. This is what the sample apps use. Because anyone who can set these headers can impersonate anyone, the embedding page must be served by a backend that has already authenticated the user, and Quorum must not be exposed directly to browsers.

When both are set, `auth-headers` wins and no bearer token is sent. `tenant-id` is sent as `X-Tenant-ID` alongside either.

Approve and Reject buttons only appear when the server reports that the authenticated viewer is allowed to act on the request (`viewer_can_act`). A panel with no buttons usually means the viewer is the maker, has already voted, or lacks the role for the current stage.

## Components

### `<quorum-approval-panel>`

Full detail panel for a single request. Shows status, stage progress, a formatted or raw payload, the audit timeline, and approve/reject actions with an optional comment.

```html
<quorum-approval-panel request-id="req_abc123"></quorum-approval-panel>
```

| Attribute | Default | Description |
|-----------|---------|-------------|
| `request-id` | *required* | The request ID to display |
| `api-url` | | Quorum API base URL |
| `token` | | Bearer token (without the `Bearer ` prefix) |
| `tenant-id` | | Tenant identifier |
| `auth-headers` | | JSON string of custom auth headers |
| `poll-interval` | `30000` | Polling interval in ms, used when SSE is disabled or disconnects |
| `sse` | `true` | Set to `"false"` to disable Server-Sent Events and poll only |
| `suppress-errors` | | When present, hides inline error messages |

The Payload tab shows a label-value view when the request carries [display template](https://github.com/lawale/quorum#display-templates) metadata, with a toggle to the raw JSON. When the template defines a `changes` block, it also renders a before/after table (styleable via `::part(changes)`) with changed values highlighted.

### `<quorum-request-list>`

Paginated, filterable table of requests. Clicking a row emits `quorum:select`.

```html
<quorum-request-list status="pending" page-size="5"></quorum-request-list>
```

| Attribute | Default | Description |
|-----------|---------|-------------|
| `api-url` | | Quorum API base URL |
| `token` | | Bearer token (without the `Bearer ` prefix) |
| `tenant-id` | | Tenant identifier |
| `auth-headers` | | JSON string of custom auth headers |
| `status` | | Filter by status (`pending`, `approved`, `rejected`, `cancelled`, `expired`) |
| `type` | | Filter by request type (matches policy type) |
| `page-size` | `10` | Number of requests per page |

### `<quorum-stage-progress>`

Compact stage indicator for multi-stage approval workflows.

```html
<quorum-stage-progress request-id="req_abc123"></quorum-stage-progress>
```

| Attribute | Default | Description |
|-----------|---------|-------------|
| `request-id` | *required* | The request ID to track |
| `api-url` | | Quorum API base URL |
| `token` | | Bearer token (without the `Bearer ` prefix) |
| `tenant-id` | | Tenant identifier |
| `auth-headers` | | JSON string of custom auth headers |
| `poll-interval` | `30000` | Polling interval in ms, used when SSE is disabled or disconnects |
| `sse` | `true` | Set to `"false"` to disable Server-Sent Events and poll only |

When an approval panel and a stage progress element for the same request are on one page, the progress element refreshes immediately after an approve or reject, without waiting for SSE or the next poll.

## Real-time updates

The approval panel and stage progress elements open an SSE stream to `/requests/{id}/events` while the request is pending and reload on every event. The stream is a streaming `fetch` rather than `EventSource`, which is what allows custom auth headers; it also means the host page's origin must be allowed by the Quorum server's CORS settings. If the stream cannot be opened or drops, the element falls back to polling at `poll-interval`. Once a request leaves the pending state, both SSE and polling stop.

## Events

All events bubble and are `composed`, so they can be handled on the element, any ancestor, or `document`.

| Event | Emitted by | `detail` |
|-------|------------|----------|
| `quorum:approved` | approval panel | `{ requestId, request }` |
| `quorum:rejected` | approval panel | `{ requestId, request }` |
| `quorum:select` | request list | `{ requestId, request }` |
| `quorum:error` | approval panel, request list | `{ action?, message, status }` where `action` is `load`, `approve`, or `reject` |

A typical wiring: navigate on row click, refresh after a decision, surface errors as a toast.

```js
document.addEventListener('quorum:select', (e) => {
  location.href = `/requests/${e.detail.requestId}`;
});

const panel = document.querySelector('quorum-approval-panel');
panel.addEventListener('quorum:approved', () => location.reload());
panel.addEventListener('quorum:rejected', () => location.reload());
panel.addEventListener('quorum:error', (e) => showToast(e.detail.message));
```

## Theming

Every element reads CSS custom properties with built-in fallbacks. Custom properties inherit through shadow roots, so set them on the element, a wrapper, or `:root` to restyle everything at once.

```css
:root {
  --quorum-font: 'IBM Plex Sans', sans-serif;
  --quorum-accent: #0ea5e9;
  --quorum-radius: 4px;
}

/* dark mode */
.dark quorum-approval-panel,
.dark quorum-request-list {
  --quorum-bg: #18181b;
  --quorum-fg: #fafafa;
  --quorum-muted: #a1a1aa;
  --quorum-surface: #27272a;
  --quorum-border: #3f3f46;
  --quorum-divider: #27272a;
  --quorum-track: #3f3f46;
  --quorum-shadow: none;
}
```

| Property | Default | Used for |
|----------|---------|----------|
| `--quorum-font` | Inter, system sans-serif | Base font family |
| `--quorum-font-mono` | system monospace | IDs, payload, audit details |
| `--quorum-bg` | `#fff` | Panel and list background |
| `--quorum-fg` | `#191c1d` | Primary text |
| `--quorum-muted` | `#464555` | Secondary text and labels |
| `--quorum-surface` | `#f3f4f5` | Table header, payload box, hover rows |
| `--quorum-border` | `rgba(199,196,216,.4)` | Inputs, buttons, stage indicators |
| `--quorum-divider` | `rgba(199,196,216,.15)` | Section and tab separators |
| `--quorum-track` | `#edeeef` | Unfilled stage connectors and timeline rail |
| `--quorum-accent` | `#4f46e5` | Active tab, current stage, input focus |
| `--quorum-success` / `--quorum-success-hover` | `#10b981` / `#059669` | Approve button, completed stages |
| `--quorum-danger` / `--quorum-danger-hover` | `#ef4444` / `#dc2626` | Reject button, failed stages, errors |
| `--quorum-radius` / `--quorum-radius-sm` | `8px` / `6px` | Container and control corners |
| `--quorum-shadow` | soft drop shadow | Panel and list elevation |
| `--quorum-panel-max-width` | `600px` | Approval panel width cap |
| `--quorum-status-<status>-bg` / `-fg` | per status | Badge colours, where `<status>` is `pending`, `approved`, `rejected`, `cancelled`, or `expired` |
| `--quorum-action-<action>` | per action | Audit timeline dot and label colour, e.g. `--quorum-action-approved` |

For structural changes beyond colour and type, the elements expose `::part()` selectors:

```css
quorum-approval-panel::part(panel) { max-width: none; }
quorum-approval-panel::part(approve-button) { text-transform: uppercase; }
quorum-request-list::part(row):hover { outline: 2px solid var(--quorum-accent); }
quorum-stage-progress::part(badge-pending) { font-weight: 700; }
```

| Element | Parts |
|---------|-------|
| `quorum-approval-panel` | `panel`, `header`, `tabs`, `tab`, `tab-content`, `payload`, `actions`, `comment`, `button`, `approve-button`, `reject-button`, `error`, `badge`, `badge-<status>`, `stage-bar`, `stage`, `stage-<state>`, `stage-indicator`, `timeline` |
| `quorum-request-list` | `list`, `table`, `row`, `pagination`, `error`, `badge`, `badge-<status>` |
| `quorum-stage-progress` | `progress`, `header`, `badge`, `badge-<status>`, `stage-bar`, `stage`, `stage-<state>`, `stage-indicator` |

`<state>` is one of `complete`, `current`, `upcoming`, or `failed`.

## JavaScript API

### `configure(config)`

Set global defaults for all widgets. Each value may be static or a getter, which is called on every request so rotating tokens and per-session tenants just work.

```js
import { configure } from '@oluwavader/quorum-embed';

configure({
  apiUrl: '/api',
  token: () => localStorage.getItem('auth_token') ?? '',
  tenantId: () => currentUser.tenantId,
  authHeaders: { 'X-Custom': 'value' },
});
```

### `createClient(config)`

The same API client the elements use, for calls outside of them.

```js
import { createClient } from '@oluwavader/quorum-embed';

const client = createClient({
  apiUrl: 'https://your-quorum-server.com',
  tenantId: 'your-tenant-id',
  token: 'your-token',
});

const request = await client.getRequest('req_abc123');
const { data, total } = await client.listRequests({ status: 'pending', page: 1, per_page: 10 });
const logs = await client.getAudit('req_abc123');

await client.approve('req_abc123', 'Looks good');
await client.reject('req_abc123', 'Missing documentation');
```

Errors are thrown as `ApiError` with `message` and `status`.

### Element classes

The element classes are exported for typed access and subclassing. Each registers its tag on import.

```ts
import { QuorumApprovalPanel } from '@oluwavader/quorum-embed';

const panel = document.querySelector('quorum-approval-panel') as QuorumApprovalPanel;
panel.requestId = 'req_abc123'; // same as setting the request-id attribute
```

## Browser support

Any evergreen browser. The bundle ships no polyfills and relies on Custom Elements, Shadow DOM, `fetch` streaming, and `color-mix()`.

## License

MIT - See [Quorum](https://github.com/lawale/quorum) for full documentation, server setup, and examples.
