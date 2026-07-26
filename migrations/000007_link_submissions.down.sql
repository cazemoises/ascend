DROP INDEX IF EXISTS idx_submissions_user_created;
ALTER TABLE submissions DROP COLUMN user_id;
