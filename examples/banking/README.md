# Banking App — Quorum Example

A sample wire transfer application built with Go that integrates with Quorum for multi-stage approval, using the [Quorum Go client](../../clients/go).

## What it demonstrates

- **Multi-stage approval**: Wire transfers require manager review, then compliance check
- **The Go client**: `CreateRequest` with the transfer's creator as maker and an idempotency key, so a retried submission never opens a second approval
- **Webhook callbacks**: Quorum calls back when a decision is made; the app executes the transfer
- **HMAC-SHA256 signature verification**: `client.ParseWebhook` rejects any delivery not signed with the shared secret

The `wire_transfer` policy itself is registered by `scripts/setup.sh`.

## Running

```bash
# Standalone (requires Quorum running on :8080)
go run .

# With Docker Compose (from project root)
docker compose --profile demo up

# Build the image on its own (from project root; the example depends on the
# client in this repository)
docker build -f examples/banking/Dockerfile .
```

Visit [http://localhost:3001](http://localhost:3001) to use the app.

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `QUORUM_API_URL` | `http://localhost:8080` | Quorum server URL |
| `SELF_URL` | `http://localhost:3001` | Publicly reachable URL of this app |
| `WEBHOOK_SECRET` | `banking-webhook-secret` | Shared secret for webhook HMAC verification |
| `PORT` | `3001` | HTTP listen port |

## Flow

1. User creates a wire transfer on the banking app
2. App submits a `wire_transfer` request to Quorum
3. A manager approves in the Quorum console (Stage 1)
4. A compliance officer approves (Stage 2)
5. Quorum sends a webhook to the banking app
6. Banking app verifies the HMAC signature and marks the transfer as executed
