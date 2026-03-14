CREATE TABLE IF NOT EXISTS requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(255) UNIQUE,
    type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    maker_id VARCHAR(255) NOT NULL,
    callback_url VARCHAR(2048),
    metadata JSONB,
    fingerprint VARCHAR(64),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_requests_status ON requests (status);
CREATE INDEX idx_requests_type ON requests (type);
CREATE INDEX idx_requests_maker_id ON requests (maker_id);
CREATE INDEX idx_requests_fingerprint ON requests (type, fingerprint) WHERE status = 'pending';
