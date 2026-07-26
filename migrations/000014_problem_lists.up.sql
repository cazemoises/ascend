-- Weekly problem lists: teacher-published content with self-declared student
-- progress, deliberately separate from challenges/submissions — these items
-- aren't judge-compatible (persistent state, multi-method classes, file side
-- effects), so there is no automated grading here at all.
CREATE TABLE problem_lists (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  teacher_id UUID NOT NULL REFERENCES users(id),
  title TEXT NOT NULL,
  week_label TEXT,
  description TEXT,
  published BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE list_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  list_id UUID NOT NULL REFERENCES problem_lists(id) ON DELETE CASCADE,
  ordinal INT NOT NULL,
  title TEXT NOT NULL,
  difficulty TEXT NOT NULL CHECK (difficulty = ANY (ARRAY['easy','medium','hard'])),
  is_bonus BOOLEAN NOT NULL DEFAULT false,
  body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_list_items_list_ordinal ON list_items(list_id, ordinal);

CREATE TABLE list_item_completions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  list_item_id UUID NOT NULL REFERENCES list_items(id) ON DELETE CASCADE,
  student_id UUID NOT NULL REFERENCES users(id),
  completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (list_item_id, student_id)
);
