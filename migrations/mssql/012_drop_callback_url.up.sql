-- Remove deprecated callback_url column from requests.
-- Webhook delivery is now handled exclusively via tenant-registered webhooks.
IF COL_LENGTH('[quorum].[requests]', 'callback_url') IS NOT NULL
BEGIN
    ALTER TABLE [quorum].[requests] DROP COLUMN callback_url;
END
