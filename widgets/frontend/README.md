# @oluwavader/quorum-embed

Embeddable Web Components for the [Quorum](https://github.com/lawale/quorum) approval engine. Drop approval workflows into any web app with zero framework dependencies.

## Install

```bash
npm install @oluwavader/quorum-embed
```

Or load directly via script tag (IIFE bundle):

```html
<script src="https://unpkg.com/@oluwavader/quorum-embed/dist/embed.js"></script>
```

## Quick Setup

Configure global defaults before using any components:

```js
import { configure } from '@oluwavader/quorum-embed';

configure({
  apiUrl: 'https://your-quorum-server.com',
  tenantId: 'your-tenant-id',
  token: () => getAuthToken(), // static string or getter function
});
```

Per-element HTML attributes override global config.

## Components

### `<quorum-approval-panel>`

Full detail panel for a single request. Shows status, stage progress, audit timeline, and approve/reject actions.

```html
<quorum-approval-panel request-id="req_abc123"></quorum-approval-panel>
```

| Attribute | Default | Description |
|-----------|---------|-------------|
| `request-id` | *required* | The request ID to display |
| `api-url` | | Quorum API base URL |
| `token` | | Bearer token for authentication |
| `tenant-id` | | Tenant identifier |
| `auth-headers` | | JSON string of custom auth headers |
| `poll-interval` | `30000` | Polling interval in ms (fallback when SSE unavailable) |
| `sse` | `true` | Enable Server-Sent Events for real-time updates |
| `suppress-errors` | | When present, hides inline error messages |

### `<quorum-request-list>`

Paginated, filterable list of approval requests.

```html
<quorum-request-list status="pending" page-size="5"></quorum-request-list>
```

| Attribute | Default | Description |
|-----------|---------|-------------|
| `api-url` | | Quorum API base URL |
| `token` | | Bearer token for authentication |
| `tenant-id` | | Tenant identifier |
| `auth-headers` | | JSON string of custom auth headers |
| `status` | | Filter by status (`pending`, `approved`, `rejected`, `cancelled`, `expired`) |
| `type` | | Filter by request type (matches policy type) |
| `page-size` | `10` | Number of requests per page |

### `<quorum-stage-progress>`

Compact stage progress indicator for multi-stage approval workflows.

```html
<quorum-stage-progress request-id="req_abc123"></quorum-stage-progress>
```

| Attribute | Default | Description |
|-----------|---------|-------------|
| `request-id` | *required* | The request ID to track |
| `api-url` | | Quorum API base URL |
| `token` | | Bearer token for authentication |
| `tenant-id` | | Tenant identifier |
| `auth-headers` | | JSON string of custom auth headers |
| `poll-interval` | `30000` | Polling interval in ms (fallback when SSE unavailable) |
| `sse` | `true` | Enable Server-Sent Events for real-time updates |

## JavaScript API

### `configure(config)`

Set global defaults for all widgets. Supports static values or getter functions for dynamic session data.

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

Programmatic API client for use outside of web components.

```js
import { createClient } from '@oluwavader/quorum-embed';

const client = createClient({
  apiUrl: 'https://your-quorum-server.com',
  tenantId: 'your-tenant-id',
  token: 'your-token',
});

const request = await client.getRequest('req_abc123');
const { data, total } = await client.listRequests({ status: 'pending', page: 1, pageSize: 10 });

await client.approve('req_abc123', 'Looks good');
await client.reject('req_abc123', 'Missing documentation');
```

## License

MIT - See [Quorum](https://github.com/lawale/quorum) for full documentation, server setup, and examples.
