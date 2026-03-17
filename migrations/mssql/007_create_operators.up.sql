CREATE TABLE [quorum].[operators] (
    id UNIQUEIDENTIFIER PRIMARY KEY,
    username NVARCHAR(255) NOT NULL,
    password_hash NVARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL DEFAULT '',
    must_change_password BIT NOT NULL DEFAULT 0,
    created_at DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    updated_at DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    CONSTRAINT uq_operators_username UNIQUE (username)
);
