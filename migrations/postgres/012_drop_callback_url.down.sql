-- Re-add callback_url column to requests.
ALTER TABLE quorum.requests ADD COLUMN callback_url VARCHAR(2048);
