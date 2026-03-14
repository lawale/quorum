ALTER TABLE [quorum].[policies] ADD permission_check_url NVARCHAR(2048);
ALTER TABLE [quorum].[requests] ADD eligible_reviewers NVARCHAR(MAX);
