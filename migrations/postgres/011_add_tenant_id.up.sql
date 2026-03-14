-- Tenants table
CREATE TABLE IF NOT EXISTS quorum.tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed the default tenant
INSERT INTO quorum.tenants (slug, name) VALUES ('default', 'Default Tenant');

-- Add tenant_id to policies
ALTER TABLE quorum.policies ADD COLUMN tenant_id VARCHAR(255) NOT NULL DEFAULT 'default';
ALTER TABLE quorum.policies DROP CONSTRAINT policies_request_type_key;
ALTER TABLE quorum.policies ADD CONSTRAINT policies_tenant_request_type_key UNIQUE (tenant_id, request_type);
CREATE INDEX idx_policies_tenant_id ON quorum.policies (tenant_id);

-- Add tenant_id to requests
ALTER TABLE quorum.requests ADD COLUMN tenant_id VARCHAR(255) NOT NULL DEFAULT 'default';
CREATE INDEX idx_requests_tenant_id ON quorum.requests (tenant_id);
DROP INDEX IF EXISTS idx_requests_fingerprint;
CREATE INDEX idx_requests_fingerprint ON quorum.requests (tenant_id, type, fingerprint) WHERE status = 'pending';

-- Add tenant_id to webhooks
ALTER TABLE quorum.webhooks ADD COLUMN tenant_id VARCHAR(255) NOT NULL DEFAULT 'default';
CREATE INDEX idx_webhooks_tenant_id ON quorum.webhooks (tenant_id);

-- Add tenant_id to audit_logs
ALTER TABLE quorum.audit_logs ADD COLUMN tenant_id VARCHAR(255) NOT NULL DEFAULT 'default';
CREATE INDEX idx_audit_logs_tenant_id ON quorum.audit_logs (tenant_id);
