IF OBJECT_ID('[quorum].[requests]', 'U') IS NULL
CREATE TABLE [quorum].[requests] (
   id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
   idempotency_key NVARCHAR(255) UNIQUE,
   type NVARCHAR(255) NOT NULL,
   payload NVARCHAR(MAX) NOT NULL,
   status NVARCHAR(50) NOT NULL DEFAULT 'pending',
   maker_id NVARCHAR(255) NOT NULL,
   callback_url NVARCHAR(2048),
   metadata NVARCHAR(MAX),
   fingerprint NVARCHAR(64),
   expires_at DATETIMEOFFSET,
   created_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE(),
   updated_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE()
);

CREATE INDEX idx_requests_status ON [quorum].[requests] (status);
CREATE INDEX idx_requests_type ON [quorum].[requests] (type);
CREATE INDEX idx_requests_maker_id ON [quorum].[requests] (maker_id);
CREATE INDEX idx_requests_fingerprint ON [quorum].[requests] (type, fingerprint) WHERE status = 'pending';
