CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url VARCHAR(2048) NOT NULL,
    events JSONB NOT NULL,
    secret VARCHAR(255) NOT NULL,
    request_type VARCHAR(255),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhooks_active ON webhooks (active) WHERE active = true;
CREATE INDEX idx_webhooks_request_type ON webhooks (request_type);
