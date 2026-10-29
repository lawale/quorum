IF COL_LENGTH('quorum.requests', 'maker_attributes') IS NULL
    ALTER TABLE [quorum].[requests] ADD maker_attributes NVARCHAR(MAX);
