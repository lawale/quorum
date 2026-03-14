DROP INDEX IF EXISTS idx_webhook_outbox_pending ON [quorum].[webhook_outbox];
IF EXISTS (SELECT * FROM sys.tables t JOIN sys.schemas s ON t.schema_id = s.schema_id WHERE s.name = 'quorum' AND t.name = 'webhook_outbox')
    DROP TABLE [quorum].[webhook_outbox];
