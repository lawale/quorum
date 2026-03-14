# Expense Tracker — Quorum Example

A sample expense management application built with Node.js and Express that integrates with Quorum for approval workflows.

## What it demonstrates

- **Single-stage approval**: Expenses require one manager or admin approval
- **Status polling**: The detail page polls Quorum for the latest request status
- **Webhook callbacks**: Receives approval/rejection events via webhook with HMAC verification
- **Role-based access**: Only `manager` and `admin` roles can approve expenses
- **Policy self-registration**: The app registers its policy with Quorum on startup

## Running

```bash
# Standalone (requires Quorum running on :8080)
npm install
npm start

# With Docker Compose (from project root)
docker compose --profile demo up
```

Visit [http://localhost:3002](http://localhost:3002) to use the app.

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `QUORUM_API_URL` | `http://localhost:8080` | Quorum server URL |
| `SELF_URL` | `http://localhost:3002` | Publicly reachable URL of this app |
| `WEBHOOK_SECRET` | `expenses-webhook-secret` | Shared secret for webhook HMAC verification |
| `PORT` | `3002` | HTTP listen port |

## Flow

1. Employee submits an expense on the expense tracker
2. App submits an `expense_approval` request to Quorum
3. A manager approves in the Quorum console
4. Quorum sends a webhook (or the app polls for status)
5. Expense tracker updates the expense status
