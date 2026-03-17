CREATE TABLE IF NOT EXISTS quorum.webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL DEFAULT 'default',
    url VARCHAR(2048) NOT NULL,
    events JSONB NOT NULL,
    secret VARCHAR(255) NOT NULL,
    request_type VARCHAR(255),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhooks_active ON quorum.webhooks (active) WHERE active = true;
CREATE INDEX idx_webhooks_request_type ON quorum.webhooks (request_type);
CREATE INDEX idx_webhooks_tenant_id ON quorum.webhooks (tenant_id);
