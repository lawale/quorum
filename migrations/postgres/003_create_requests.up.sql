CREATE TABLE IF NOT EXISTS quorum.requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL DEFAULT 'default',
    idempotency_key VARCHAR(255) UNIQUE,
    type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    maker_id VARCHAR(255) NOT NULL,
    eligible_reviewers JSONB,
    metadata JSONB,
    fingerprint VARCHAR(64),
    current_stage INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_requests_status ON quorum.requests (status);
CREATE INDEX idx_requests_type ON quorum.requests (type);
CREATE INDEX idx_requests_maker_id ON quorum.requests (maker_id);
CREATE INDEX idx_requests_tenant_id ON quorum.requests (tenant_id);
CREATE UNIQUE INDEX idx_requests_fingerprint ON quorum.requests (tenant_id, type, fingerprint) WHERE status = 'pending';
