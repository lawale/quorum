-- Re-add callback_url column to requests.
ALTER TABLE [quorum].[requests] ADD callback_url NVARCHAR(2048);
