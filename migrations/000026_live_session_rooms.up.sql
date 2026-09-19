CREATE TABLE live_session_rooms (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id UUID NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
  list_item_id UUID NOT NULL REFERENCES list_items(id),
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'frozen')),
  doc_snapshot BYTEA NOT NULL DEFAULT ''::bytea,
  text_snapshot TEXT NOT NULL DEFAULT '',
  snapshot_updated_at TIMESTAMPTZ NULL,
  frozen_at TIMESTAMPTZ NULL,
  frozen_reason TEXT NULL CHECK (frozen_reason IN ('time_limit', 'all_done', 'teacher_forced')),
  UNIQUE (session_id, list_item_id)
);
CREATE INDEX idx_live_session_rooms_session ON live_session_rooms (session_id, status);

CREATE TABLE live_room_participants (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id UUID NOT NULL REFERENCES live_session_rooms(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id),
  joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  left_at TIMESTAMPTZ NULL,
  marked_done BOOLEAN NOT NULL DEFAULT false
);
CREATE INDEX idx_live_room_participants_room_present ON live_room_participants (room_id) WHERE left_at IS NULL;
CREATE INDEX idx_live_room_participants_room_user ON live_room_participants (room_id, user_id);

ALTER TABLE submissions ADD COLUMN live_room_id UUID NULL REFERENCES live_session_rooms(id);
CREATE INDEX idx_submissions_live_room ON submissions (live_room_id, created_at DESC);
