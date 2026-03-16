-- Remove deprecated callback_url column from requests.
-- Webhook delivery is now handled exclusively via tenant-registered webhooks.
ALTER TABLE quorum.requests DROP COLUMN IF EXISTS callback_url;
