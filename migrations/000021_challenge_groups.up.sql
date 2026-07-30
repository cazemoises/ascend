CREATE TABLE challenge_groups (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  teacher_id UUID        NOT NULL REFERENCES users(id),
  title      TEXT        NOT NULL,
  ordinal    INT         NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE challenge_collections ADD COLUMN group_id UUID NULL
  REFERENCES challenge_groups(id) ON DELETE SET NULL;
