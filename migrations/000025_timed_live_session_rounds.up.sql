ALTER TABLE live_sessions
  ADD COLUMN mode TEXT NOT NULL DEFAULT 'individual'
  CHECK (mode IN ('individual', 'timed_challenge', 'collaborative'));

CREATE TABLE live_session_rounds (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id UUID NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
  list_item_id UUID NOT NULL REFERENCES list_items(id),
  duration_seconds INT NOT NULL DEFAULT 600 CHECK (duration_seconds > 0),
  started_at TIMESTAMPTZ NULL,
  ended_at TIMESTAMPTZ NULL,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'ended')),
  sequence INT NOT NULL CHECK (sequence >= 0),
  UNIQUE (session_id, list_item_id),
  UNIQUE (session_id, sequence)
);

CREATE UNIQUE INDEX idx_live_session_rounds_one_active
  ON live_session_rounds (session_id) WHERE status = 'active';
CREATE INDEX idx_live_session_rounds_session_sequence
  ON live_session_rounds (session_id, sequence);
