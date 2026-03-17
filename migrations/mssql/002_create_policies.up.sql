IF OBJECT_ID('[quorum].[policies]', 'U') IS NULL
CREATE TABLE [quorum].[policies] (
    id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
    tenant_id NVARCHAR(255) NOT NULL DEFAULT 'default',
    name NVARCHAR(255) NOT NULL,
    request_type NVARCHAR(255) NOT NULL,
    stages NVARCHAR(MAX) NOT NULL,
    identity_fields NVARCHAR(MAX),
    dynamic_authorization_url NVARCHAR(2048),
    auto_expire_duration NVARCHAR(64),
    display_template NVARCHAR(MAX),
    created_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE(),
    updated_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE()
);

CREATE UNIQUE INDEX idx_policies_tenant_request_type ON [quorum].[policies] (tenant_id, request_type);
CREATE INDEX idx_policies_request_type ON [quorum].[policies] (request_type);
CREATE INDEX idx_policies_tenant_id ON [quorum].[policies] (tenant_id);
