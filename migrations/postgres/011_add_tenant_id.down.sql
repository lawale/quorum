DROP INDEX IF EXISTS idx_audit_logs_tenant_id;
ALTER TABLE quorum.audit_logs DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_webhooks_tenant_id;
ALTER TABLE quorum.webhooks DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_requests_fingerprint;
DROP INDEX IF EXISTS idx_requests_tenant_id;
ALTER TABLE quorum.requests DROP COLUMN IF EXISTS tenant_id;
CREATE INDEX idx_requests_fingerprint ON quorum.requests (type, fingerprint) WHERE status = 'pending';

ALTER TABLE quorum.policies DROP CONSTRAINT IF EXISTS policies_tenant_request_type_key;
DROP INDEX IF EXISTS idx_policies_tenant_id;
ALTER TABLE quorum.policies DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE quorum.policies ADD CONSTRAINT policies_request_type_key UNIQUE (request_type);

DROP TABLE IF EXISTS quorum.tenants;
