-- Groups challenges by named collection (theme/cycle) — order only, no
-- dates, unlike problem_lists' week-based grouping.
CREATE TABLE challenge_collections (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  teacher_id UUID        NOT NULL REFERENCES users(id),
  title      TEXT        NOT NULL,
  ordinal    INT         NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ON DELETE SET NULL: deleting a collection un-links its challenges rather
-- than deleting them (challenges have no owner in this system, so a
-- collection's teacher_id is not a reason to cascade-delete anyone's work).
ALTER TABLE challenges ADD COLUMN IF NOT EXISTS collection_id UUID NULL
  REFERENCES challenge_collections(id) ON DELETE SET NULL;
