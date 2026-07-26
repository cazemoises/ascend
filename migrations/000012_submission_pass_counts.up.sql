-- Expose "X of Y test cases passed" without persisting per-case output.
-- Both nullable: submissions judged before this migration (and any that fail
-- before the worker runs a single case) stay NULL, so no backfill is needed.
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS passed_count     INT NULL;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS total_test_cases INT NULL;
