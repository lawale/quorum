# Access Portal — Quorum Example

A sample system access request portal built with Python and Flask that integrates with Quorum for threshold-based security review.

## What it demonstrates

- **Threshold-based voting**: Requires 2 of 3 security reviewers to approve
- **Eligible reviewers**: Requests are scoped to specific security team members
- **Webhook callbacks**: Receives approval/rejection events with HMAC verification
- **Status polling**: Detail page fetches latest status from Quorum
- **Policy self-registration**: The app registers its policy with Quorum on startup

## Running

```bash
# Standalone (requires Quorum running on :8080)
pip install -r requirements.txt
python app.py

# With Docker Compose (from project root)
docker compose --profile demo up
```

Visit [http://localhost:3003](http://localhost:3003) to use the app.

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `QUORUM_API_URL` | `http://localhost:8080` | Quorum server URL |
| `SELF_URL` | `http://localhost:3003` | Publicly reachable URL of this app |
| `WEBHOOK_SECRET` | `access-webhook-secret` | Shared secret for webhook HMAC verification |
| `PORT` | `3003` | HTTP listen port |

## Flow

1. User requests system access on the portal
2. App submits an `access_request` to Quorum with a list of eligible security reviewers
3. Security team members review and vote in the Quorum console
4. When 2 of 3 approve, Quorum fires an "approved" webhook
5. Portal marks the access as "granted" and (in production) would provision the access
