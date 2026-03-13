IF NOT EXISTS (SELECT * FROM sysobjects WHERE name='audit_logs' AND xtype='U')
CREATE TABLE audit_logs (
   id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
   request_id UNIQUEIDENTIFIER NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
   action NVARCHAR(50) NOT NULL,
   actor_id NVARCHAR(255) NOT NULL,
   details NVARCHAR(MAX),
   created_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE()
);

CREATE INDEX idx_audit_logs_request_id ON audit_logs (request_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at);
