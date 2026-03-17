IF NOT EXISTS (SELECT * FROM sys.tables t JOIN sys.schemas s ON t.schema_id = s.schema_id WHERE s.name = 'quorum' AND t.name = 'webhook_outbox')
CREATE TABLE [quorum].[webhook_outbox] (
    id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
    request_id UNIQUEIDENTIFIER NOT NULL REFERENCES [quorum].[requests](id),
    webhook_url NVARCHAR(2048) NOT NULL,
    webhook_secret NVARCHAR(255) NOT NULL DEFAULT '',
    payload NVARCHAR(MAX) NOT NULL,
    status NVARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 3,
    last_error NVARCHAR(MAX),
    next_retry_at DATETIMEOFFSET NOT NULL DEFAULT SYSDATETIMEOFFSET(),
    created_at DATETIMEOFFSET NOT NULL DEFAULT SYSDATETIMEOFFSET(),
    delivered_at DATETIMEOFFSET
);

CREATE INDEX idx_webhook_outbox_claimable ON [quorum].[webhook_outbox] (next_retry_at)
    WHERE status IN ('pending', 'processing');
