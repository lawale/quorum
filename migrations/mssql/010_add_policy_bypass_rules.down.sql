IF COL_LENGTH('quorum.policies', 'bypass_rules') IS NOT NULL
    ALTER TABLE [quorum].[policies] DROP COLUMN bypass_rules;
