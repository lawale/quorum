-- Drop tenant_id from audit_logs
DROP INDEX IF EXISTS idx_audit_logs_tenant_id ON [quorum].[audit_logs];
DECLARE @df_audit NVARCHAR(256);
SELECT @df_audit = d.name FROM sys.default_constraints d
JOIN sys.columns c ON d.parent_object_id = c.object_id AND d.parent_column_id = c.column_id
WHERE d.parent_object_id = OBJECT_ID('[quorum].[audit_logs]') AND c.name = 'tenant_id';
IF @df_audit IS NOT NULL EXEC('ALTER TABLE [quorum].[audit_logs] DROP CONSTRAINT [' + @df_audit + ']');
IF EXISTS (SELECT 1 FROM sys.columns WHERE object_id = OBJECT_ID('[quorum].[audit_logs]') AND name = 'tenant_id')
    ALTER TABLE [quorum].[audit_logs] DROP COLUMN tenant_id;
GO

-- Drop tenant_id from webhooks
DROP INDEX IF EXISTS idx_webhooks_tenant_id ON [quorum].[webhooks];
DECLARE @df_webhooks NVARCHAR(256);
SELECT @df_webhooks = d.name FROM sys.default_constraints d
JOIN sys.columns c ON d.parent_object_id = c.object_id AND d.parent_column_id = c.column_id
WHERE d.parent_object_id = OBJECT_ID('[quorum].[webhooks]') AND c.name = 'tenant_id';
IF @df_webhooks IS NOT NULL EXEC('ALTER TABLE [quorum].[webhooks] DROP CONSTRAINT [' + @df_webhooks + ']');
IF EXISTS (SELECT 1 FROM sys.columns WHERE object_id = OBJECT_ID('[quorum].[webhooks]') AND name = 'tenant_id')
    ALTER TABLE [quorum].[webhooks] DROP COLUMN tenant_id;
GO

-- Drop tenant_id from requests, restore original fingerprint index
DROP INDEX IF EXISTS idx_requests_fingerprint ON [quorum].[requests];
DROP INDEX IF EXISTS idx_requests_tenant_id ON [quorum].[requests];
DECLARE @df_requests NVARCHAR(256);
SELECT @df_requests = d.name FROM sys.default_constraints d
JOIN sys.columns c ON d.parent_object_id = c.object_id AND d.parent_column_id = c.column_id
WHERE d.parent_object_id = OBJECT_ID('[quorum].[requests]') AND c.name = 'tenant_id';
IF @df_requests IS NOT NULL EXEC('ALTER TABLE [quorum].[requests] DROP CONSTRAINT [' + @df_requests + ']');
IF EXISTS (SELECT 1 FROM sys.columns WHERE object_id = OBJECT_ID('[quorum].[requests]') AND name = 'tenant_id')
    ALTER TABLE [quorum].[requests] DROP COLUMN tenant_id;
CREATE INDEX idx_requests_fingerprint ON [quorum].[requests] (type, fingerprint) WHERE status = 'pending';
GO

-- Drop tenant_id from policies, restore original unique constraint
DROP INDEX IF EXISTS idx_policies_tenant_request_type ON [quorum].[policies];
DROP INDEX IF EXISTS idx_policies_tenant_id ON [quorum].[policies];
DECLARE @df_policies NVARCHAR(256);
SELECT @df_policies = d.name FROM sys.default_constraints d
JOIN sys.columns c ON d.parent_object_id = c.object_id AND d.parent_column_id = c.column_id
WHERE d.parent_object_id = OBJECT_ID('[quorum].[policies]') AND c.name = 'tenant_id';
IF @df_policies IS NOT NULL EXEC('ALTER TABLE [quorum].[policies] DROP CONSTRAINT [' + @df_policies + ']');
IF EXISTS (SELECT 1 FROM sys.columns WHERE object_id = OBJECT_ID('[quorum].[policies]') AND name = 'tenant_id')
    ALTER TABLE [quorum].[policies] DROP COLUMN tenant_id;
ALTER TABLE [quorum].[policies] ADD CONSTRAINT UQ_policies_request_type UNIQUE (request_type);
GO

-- Drop tenants table
IF EXISTS (SELECT * FROM sys.tables t JOIN sys.schemas s ON t.schema_id = s.schema_id WHERE s.name = 'quorum' AND t.name = 'tenants')
    DROP TABLE [quorum].[tenants];
