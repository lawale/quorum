IF OBJECT_ID('[quorum].[audit_logs]', 'U') IS NULL
CREATE TABLE [quorum].[audit_logs] (
    id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
    tenant_id NVARCHAR(255) NOT NULL,
    request_id UNIQUEIDENTIFIER NOT NULL REFERENCES [quorum].[requests](id) ON DELETE CASCADE,
    action NVARCHAR(50) NOT NULL,
    actor_id NVARCHAR(255) NOT NULL,
    details NVARCHAR(MAX),
    created_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE()
);

CREATE INDEX idx_audit_logs_request_id ON [quorum].[audit_logs] (request_id);
CREATE INDEX idx_audit_logs_created_at ON [quorum].[audit_logs] (created_at);
CREATE INDEX idx_audit_logs_tenant_id ON [quorum].[audit_logs] (tenant_id);
