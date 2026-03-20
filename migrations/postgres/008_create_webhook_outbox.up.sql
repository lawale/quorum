CREATE TABLE IF NOT EXISTS quorum.webhook_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES quorum.requests(id) ON DELETE CASCADE,
    webhook_url VARCHAR(2048) NOT NULL,
    webhook_secret VARCHAR(255) NOT NULL DEFAULT '',
    payload JSONB NOT NULL,
    event_type VARCHAR(50) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 3,
    last_error TEXT,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ
);

CREATE INDEX idx_webhook_outbox_claimable ON quorum.webhook_outbox (next_retry_at)
    WHERE status IN ('pending', 'processing');
