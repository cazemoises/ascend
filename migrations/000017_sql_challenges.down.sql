ALTER TABLE test_cases DROP COLUMN IF EXISTS order_matters;

ALTER TABLE challenges
  DROP COLUMN IF EXISTS sql_schema,
  DROP COLUMN IF EXISTS language;
