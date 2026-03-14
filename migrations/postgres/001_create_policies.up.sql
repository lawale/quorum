CREATE TABLE IF NOT EXISTS policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    request_type VARCHAR(255) NOT NULL UNIQUE,
    required_approvals INT NOT NULL DEFAULT 1,
    allowed_checker_roles JSONB,
    rejection_policy VARCHAR(50) NOT NULL DEFAULT 'any',
    max_checkers INT,
    identity_fields JSONB,
    auto_expire_duration INTERVAL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_policies_request_type ON policies (request_type);
