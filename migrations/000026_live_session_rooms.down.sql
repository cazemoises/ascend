ALTER TABLE submissions DROP COLUMN IF EXISTS live_room_id;
DROP TABLE IF EXISTS live_room_participants;
DROP TABLE IF EXISTS live_session_rooms;
