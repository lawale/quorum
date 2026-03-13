IF NOT EXISTS (SELECT * FROM sysobjects WHERE name='requests' AND xtype='U')
CREATE TABLE requests (
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

CREATE INDEX idx_requests_status ON requests (status);
CREATE INDEX idx_requests_type ON requests (type);
CREATE INDEX idx_requests_maker_id ON requests (maker_id);
CREATE INDEX idx_requests_fingerprint ON requests (type, fingerprint) WHERE status = 'pending';
