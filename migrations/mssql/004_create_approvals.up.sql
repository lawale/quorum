IF OBJECT_ID('[quorum].[approvals]', 'U') IS NULL
CREATE TABLE [quorum].[approvals] (
    id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
    request_id UNIQUEIDENTIFIER NOT NULL REFERENCES [quorum].[requests](id) ON DELETE CASCADE,
    checker_id NVARCHAR(255) NOT NULL,
    decision NVARCHAR(50) NOT NULL,
    comment NVARCHAR(MAX),
    stage_index INT NOT NULL DEFAULT 0,
    created_at DATETIMEOFFSET NOT NULL DEFAULT GETUTCDATE(),

    CONSTRAINT approvals_request_id_checker_id_stage_idx_key UNIQUE (request_id, checker_id, stage_index)
);

CREATE INDEX idx_approvals_request_id ON [quorum].[approvals] (request_id);
