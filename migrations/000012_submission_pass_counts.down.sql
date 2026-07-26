ALTER TABLE submissions
  DROP COLUMN IF EXISTS passed_count,
  DROP COLUMN IF EXISTS total_test_cases;
