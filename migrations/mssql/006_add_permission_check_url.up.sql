ALTER TABLE policies ADD permission_check_url NVARCHAR(2048);
ALTER TABLE requests ADD eligible_reviewers NVARCHAR(MAX);
