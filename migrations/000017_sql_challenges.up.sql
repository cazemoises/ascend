-- Enables SQL as a challenge modality. language is nullable: NULL keeps
-- today's behavior (multi-language, student picks python/go/javascript per
-- submission); 'sql' marks the challenge as SQL-only, gating the student
-- editor to a single query field and the shared DDL/seed schema below.
ALTER TABLE challenges ADD COLUMN IF NOT EXISTS language   TEXT NULL;
ALTER TABLE challenges ADD COLUMN IF NOT EXISTS sql_schema TEXT NULL;

-- Per-test-case override: when true, the judge compares the student's
-- output against expected_output line-for-line as-is (order matters, e.g.
-- exercises that specifically test ORDER BY). Defaults to false, matching
-- the SQL-challenge default of comparing result sets as unordered multisets.
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS order_matters BOOLEAN NOT NULL DEFAULT false;
