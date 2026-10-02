DROP TABLE live_room_updates;
ALTER TABLE live_session_rooms
  DROP COLUMN doc_baseline,
  DROP COLUMN doc_version,
  DROP COLUMN snapshot_version;
