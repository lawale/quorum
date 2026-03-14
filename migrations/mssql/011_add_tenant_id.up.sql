-- Tenants table
IF NOT EXISTS (SELECT * FROM sys.tables t JOIN sys.schemas s ON t.schema_id = s.schema_id WHERE s.name = 'quorum' AND t.name = 'tenants')
CREATE TABLE [quorum].[tenants] (
    id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
    slug NVARCHAR(255) NOT NULL UNIQUE,
    name NVARCHAR(255) NOT NULL,
    created_at DATETIMEOFFSET NOT NULL DEFAULT SYSDATETIMEOFFSET(),
    updated_at DATETIMEOFFSET NOT NULL DEFAULT SYSDATETIMEOFFSET()
);

-- Seed the default tenant
IF NOT EXISTS (SELECT 1 FROM [quorum].[tenants] WHERE slug = 'default')
    INSERT INTO [quorum].[tenants] (slug, name) VALUES ('default', 'Default Tenant');

-- Add tenant_id to policies
IF NOT EXISTS (SELECT 1 FROM sys.columns WHERE object_id = OBJECT_ID('[quorum].[policies]') AND name = 'tenant_id')
    ALTER TABLE [quorum].[policies] ADD tenant_id NVARCHAR(255) NOT NULL DEFAULT 'default';

-- Drop the auto-generated unique constraint on request_type
DECLARE @constraint_name NVARCHAR(256);
SELECT @constraint_name = tc.CONSTRAINT_NAME
FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
JOIN INFORMATION_SCHEMA.CONSTRAINT_COLUMN_USAGE ccu ON tc.CONSTRAINT_NAME = ccu.CONSTRAINT_NAME
WHERE tc.TABLE_SCHEMA = 'quorum' AND tc.TABLE_NAME = 'policies'
  AND tc.CONSTRAINT_TYPE = 'UNIQUE' AND ccu.COLUMN_NAME = 'request_type';

IF @constraint_name IS NOT NULL
    EXEC('ALTER TABLE [quorum].[policies] DROP CONSTRAINT [' + @constraint_name + ']');

CREATE UNIQUE INDEX idx_policies_tenant_request_type ON [quorum].[policies] (tenant_id, request_type);
CREATE INDEX idx_policies_tenant_id ON [quorum].[policies] (tenant_id);

-- Add tenant_id to requests
IF NOT EXISTS (SELECT 1 FROM sys.columns WHERE object_id = OBJECT_ID('[quorum].[requests]') AND name = 'tenant_id')
    ALTER TABLE [quorum].[requests] ADD tenant_id NVARCHAR(255) NOT NULL DEFAULT 'default';

CREATE INDEX idx_requests_tenant_id ON [quorum].[requests] (tenant_id);
DROP INDEX IF EXISTS idx_requests_fingerprint ON [quorum].[requests];
CREATE INDEX idx_requests_fingerprint ON [quorum].[requests] (tenant_id, type, fingerprint) WHERE status = 'pending';

-- Add tenant_id to webhooks
IF NOT EXISTS (SELECT 1 FROM sys.columns WHERE object_id = OBJECT_ID('[quorum].[webhooks]') AND name = 'tenant_id')
    ALTER TABLE [quorum].[webhooks] ADD tenant_id NVARCHAR(255) NOT NULL DEFAULT 'default';

CREATE INDEX idx_webhooks_tenant_id ON [quorum].[webhooks] (tenant_id);

-- Add tenant_id to audit_logs
IF NOT EXISTS (SELECT 1 FROM sys.columns WHERE object_id = OBJECT_ID('[quorum].[audit_logs]') AND name = 'tenant_id')
    ALTER TABLE [quorum].[audit_logs] ADD tenant_id NVARCHAR(255) NOT NULL DEFAULT 'default';

CREATE INDEX idx_audit_logs_tenant_id ON [quorum].[audit_logs] (tenant_id);
