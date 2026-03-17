CREATE TABLE IF NOT EXISTS quorum.policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL DEFAULT 'default',
    name VARCHAR(255) NOT NULL,
    request_type VARCHAR(255) NOT NULL,
    stages JSONB NOT NULL,
    identity_fields JSONB,
    dynamic_authorization_url VARCHAR(2048),
    dynamic_authorization_secret VARCHAR(512),
    auto_expire_duration INTERVAL,
    display_template JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT policies_tenant_request_type_key UNIQUE (tenant_id, request_type)
);

CREATE INDEX idx_policies_request_type ON quorum.policies (request_type);
CREATE INDEX idx_policies_tenant_id ON quorum.policies (tenant_id);
