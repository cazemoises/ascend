CREATE TABLE live_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  problem_list_id UUID NOT NULL REFERENCES problem_lists(id),
  created_by UUID NOT NULL REFERENCES users(id),
  status TEXT NOT NULL DEFAULT 'waiting' CHECK (status IN ('waiting', 'active', 'finished')),
  min_participants INT NOT NULL CHECK (min_participants > 0),
  started_at TIMESTAMPTZ NULL,
  finished_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_live_sessions_status_created ON live_sessions (status, created_at DESC);
CREATE INDEX idx_live_sessions_creator_created ON live_sessions (created_by, created_at DESC);

CREATE TABLE live_session_participants (
  session_id UUID NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id),
  joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (session_id, user_id)
);

ALTER TABLE submissions ADD COLUMN live_session_id UUID NULL REFERENCES live_sessions(id);
CREATE INDEX idx_submissions_live_session ON submissions (live_session_id, user_id, challenge_id, created_at);
