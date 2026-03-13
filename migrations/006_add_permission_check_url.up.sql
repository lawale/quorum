ALTER TABLE policies ADD COLUMN permission_check_url VARCHAR(2048);
ALTER TABLE requests ADD COLUMN eligible_reviewers JSONB;
