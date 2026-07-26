-- Remove user_id (authentication not yet implemented)
ALTER TABLE submissions DROP COLUMN user_id;

-- Rename code to source_code
ALTER TABLE submissions RENAME COLUMN code TO source_code;

-- Convert status from submission_status enum to TEXT so the judge worker
-- can write new status values without requiring a schema migration.
ALTER TABLE submissions ALTER COLUMN status DROP DEFAULT;
ALTER TABLE submissions ALTER COLUMN status TYPE TEXT USING status::TEXT;
ALTER TABLE submissions ALTER COLUMN status SET DEFAULT 'pending';

DROP TYPE submission_status;
