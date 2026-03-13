IF NOT EXISTS (SELECT * FROM sysobjects WHERE name='approvals' AND xtype='U')
CREATE TABLE approvals (
   id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
   request_id UNIQUEIDENTIFIER NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
   checker_id NVARCHAR(255) NOT NULL,
   decision NVARCHAR(50) NOT NULL,
   comment NVARCHAR(MAX),
   created_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE(),

   UNIQUE (request_id, checker_id)
);

CREATE INDEX idx_approvals_request_id ON approvals (request_id);
