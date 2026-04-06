IF OBJECT_ID('[quorum].[requests]', 'U') IS NULL
CREATE TABLE [quorum].[requests] (
    id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
    tenant_id NVARCHAR(255) NOT NULL,
    idempotency_key NVARCHAR(255),
    type NVARCHAR(255) NOT NULL,
    payload NVARCHAR(MAX) NOT NULL,
    status NVARCHAR(50) NOT NULL DEFAULT 'pending',
    maker_id NVARCHAR(255) NOT NULL,
    eligible_reviewers NVARCHAR(MAX),
    metadata NVARCHAR(MAX),
    fingerprint NVARCHAR(64),
    current_stage INT NOT NULL DEFAULT 0,
    expires_at DATETIMEOFFSET,
    created_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE(),
    updated_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE()
);

-- Filtered unique index: enforces uniqueness for non-NULL keys while allowing multiple NULLs.
-- SQL Server's inline UNIQUE constraint treats NULL as a value, blocking multiple NULL rows.
CREATE UNIQUE INDEX idx_requests_idempotency_key ON [quorum].[requests] (idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_requests_status ON [quorum].[requests] (status);
CREATE INDEX idx_requests_type ON [quorum].[requests] (type);
CREATE INDEX idx_requests_maker_id ON [quorum].[requests] (maker_id);
CREATE INDEX idx_requests_tenant_id ON [quorum].[requests] (tenant_id);
CREATE UNIQUE INDEX idx_requests_fingerprint ON [quorum].[requests] (tenant_id, type, fingerprint) WHERE status = 'pending' AND fingerprint IS NOT NULL;

CREATE INDEX idx_requests_tenant_status ON [quorum].[requests] (tenant_id, status);
CREATE INDEX idx_requests_tenant_type ON [quorum].[requests] (tenant_id, type);
