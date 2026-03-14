CREATE TABLE IF NOT EXISTS quorum.audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES quorum.requests(id) ON DELETE CASCADE,
    action VARCHAR(50) NOT NULL,
    actor_id VARCHAR(255) NOT NULL,
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_request_id ON quorum.audit_logs (request_id);
CREATE INDEX idx_audit_logs_created_at ON quorum.audit_logs (created_at);
