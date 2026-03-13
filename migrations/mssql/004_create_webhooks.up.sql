IF NOT EXISTS (SELECT * FROM sysobjects WHERE name='webhooks' AND xtype='U')
CREATE TABLE webhooks (
   id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
   url NVARCHAR(2048) NOT NULL,
   events NVARCHAR(MAX) NOT NULL,
   secret NVARCHAR(255) NOT NULL,
   request_type NVARCHAR(255),
   active BIT NOT NULL DEFAULT 1,
   created_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE()
);

CREATE INDEX idx_webhooks_active ON webhooks (active) WHERE active = 1;
CREATE INDEX idx_webhooks_request_type ON webhooks (request_type);
