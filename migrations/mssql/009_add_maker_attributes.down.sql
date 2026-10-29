IF COL_LENGTH('quorum.requests', 'maker_attributes') IS NOT NULL
    ALTER TABLE [quorum].[requests] DROP COLUMN maker_attributes;
