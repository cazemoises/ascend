-- failed_input: the failing test case's input, only ever written when that
-- case is a sample (is_sample=true) — deliberately left NULL for a hidden
-- case so there's nothing to leak in the API response, not just something
-- the frontend chooses not to render.
-- failed_is_sample: lets the frontend distinguish "hidden case, show the
-- notice" (false) from "submission predates this column, show neither
-- block nor notice" (NULL) — both would otherwise read as the same NULL.
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS failed_input TEXT NULL;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS failed_is_sample BOOLEAN NULL;
