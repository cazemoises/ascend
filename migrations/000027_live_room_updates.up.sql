ALTER TABLE live_session_rooms
  ADD COLUMN doc_baseline BYTEA NOT NULL DEFAULT ''::bytea,
  ADD COLUMN doc_version BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN snapshot_version BIGINT NOT NULL DEFAULT 0;

-- Preserve the only available state of existing rooms. Client snapshots are
-- caches from this point onward, never a replacement for the durable journal.
UPDATE live_session_rooms SET doc_baseline = doc_snapshot;

CREATE TABLE live_room_updates (
  room_id UUID NOT NULL REFERENCES live_session_rooms(id) ON DELETE CASCADE,
  version BIGINT NOT NULL,
  payload BYTEA NOT NULL,
  PRIMARY KEY (room_id, version)
);
