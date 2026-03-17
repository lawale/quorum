IF OBJECT_ID('[quorum].[webhooks]', 'U') IS NULL
CREATE TABLE [quorum].[webhooks] (
    id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
    tenant_id NVARCHAR(255) NOT NULL DEFAULT 'default',
    url NVARCHAR(2048) NOT NULL,
    events NVARCHAR(MAX) NOT NULL,
    secret NVARCHAR(255) NOT NULL,
    request_type NVARCHAR(255),
    active BIT NOT NULL DEFAULT 1,
    created_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE()
);

CREATE INDEX idx_webhooks_active ON [quorum].[webhooks] (active) WHERE active = 1;
CREATE INDEX idx_webhooks_request_type ON [quorum].[webhooks] (request_type);
CREATE INDEX idx_webhooks_tenant_id ON [quorum].[webhooks] (tenant_id);
