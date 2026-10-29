IF COL_LENGTH('quorum.policies', 'bypass_rules') IS NULL
    ALTER TABLE [quorum].[policies] ADD bypass_rules NVARCHAR(MAX);
