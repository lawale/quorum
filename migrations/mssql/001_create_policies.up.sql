IF NOT EXISTS (SELECT * FROM sys.schemas WHERE name = 'quorum')
    EXEC('CREATE SCHEMA [quorum]');

IF OBJECT_ID('[quorum].[policies]', 'U') IS NULL
CREATE TABLE [quorum].[policies] (
   id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
   name NVARCHAR(255) NOT NULL,
   request_type NVARCHAR(255) NOT NULL UNIQUE,
   required_approvals INT NOT NULL DEFAULT 1,
   allowed_checker_roles NVARCHAR(MAX),
   rejection_policy NVARCHAR(50) NOT NULL DEFAULT 'any',
   max_checkers INT,
   identity_fields NVARCHAR(MAX),
   auto_expire_duration NVARCHAR(64),
   created_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE(),
   updated_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE()
);

CREATE INDEX idx_policies_request_type ON [quorum].[policies] (request_type);
