ALTER TABLE quorum.policies ADD COLUMN permission_check_url VARCHAR(2048);
ALTER TABLE quorum.requests ADD COLUMN eligible_reviewers JSONB;
