-- Submissions made by a teacher testing their own challenge are flagged so
-- aggregate stats (percentiles, class completion counts) can exclude them.
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS is_test_run BOOLEAN NOT NULL DEFAULT false;
