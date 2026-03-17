IF NOT EXISTS (SELECT * FROM sys.schemas WHERE name = 'quorum')
    EXEC('CREATE SCHEMA [quorum]');

IF NOT EXISTS (SELECT * FROM sys.tables t JOIN sys.schemas s ON t.schema_id = s.schema_id WHERE s.name = 'quorum' AND t.name = 'tenants')
CREATE TABLE [quorum].[tenants] (
    id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
    slug NVARCHAR(255) NOT NULL UNIQUE,
    name NVARCHAR(255) NOT NULL,
    created_at DATETIMEOFFSET NOT NULL DEFAULT SYSDATETIMEOFFSET(),
    updated_at DATETIMEOFFSET NOT NULL DEFAULT SYSDATETIMEOFFSET()
);
