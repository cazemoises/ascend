package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// LiveRoomEventsChannel carries collaborative-room websocket frames between
// API instances. Payloads are opaque (base64) — the server never decodes
// the Yjs protocol, it only relays and persists raw bytes.
const LiveRoomEventsChannel = "live_room_events"

type LiveSessionRoom struct {
	ID                string     `json:"id"`
	SessionID         string     `json:"session_id"`
	ListItemID        string     `json:"list_item_id"`
	Status            string     `json:"status"`
	TextSnapshot      string     `json:"text_snapshot"`
	SnapshotUpdatedAt *time.Time `json:"snapshot_updated_at"`
	FrozenAt          *time.Time `json:"frozen_at"`
	FrozenReason      *string    `json:"frozen_reason"`
}

type LiveRoomSummary struct {
	ListItemID     string            `json:"list_item_id"`
	ItemTitle      string            `json:"item_title"`
	ChallengeTitle *string           `json:"challenge_title"`
	RoomID         *string           `json:"room_id"`
	Status         *string           `json:"status"`
	PresentCount   int               `json:"present_count"`
	Participants   []LiveParticipant `json:"participants,omitempty"`
}

const liveRoomColumns = `id,session_id,list_item_id,status,text_snapshot,snapshot_updated_at,frozen_at,frozen_reason`

func scanLiveRoom(row interface{ Scan(...any) error }) (LiveSessionRoom, error) {
	var x LiveSessionRoom
	err := row.Scan(&x.ID, &x.SessionID, &x.ListItemID, &x.Status, &x.TextSnapshot, &x.SnapshotUpdatedAt, &x.FrozenAt, &x.FrozenReason)
	return x, err
}

// ValidateLiveRoomItem reports whether listItemID belongs to session's list
// and the session is running in collaborative mode.
func (s *Store) ValidateLiveRoomItem(ctx context.Context, sessionID, listItemID string) error {
	var ok bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM live_sessions s JOIN list_items li ON li.list_id=s.problem_list_id
		 WHERE s.id=$1 AND li.id=$2 AND s.mode='collaborative')`, sessionID, listItemID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// GetOrCreateLiveRoom creates the room on first access ("nasce sob demanda").
func (s *Store) GetOrCreateLiveRoom(ctx context.Context, sessionID, listItemID string) (LiveSessionRoom, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+liveRoomColumns+` FROM live_session_rooms WHERE session_id=$1 AND list_item_id=$2`, sessionID, listItemID)
	room, err := scanLiveRoom(row)
	if err == nil {
		return room, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return room, err
	}
	row = s.db.QueryRowContext(ctx,
		`INSERT INTO live_session_rooms(session_id,list_item_id) VALUES($1,$2)
		 ON CONFLICT (session_id,list_item_id) DO UPDATE SET session_id=live_session_rooms.session_id
		 RETURNING `+liveRoomColumns, sessionID, listItemID)
	return scanLiveRoom(row)
}

func (s *Store) GetLiveRoom(ctx context.Context, roomID string) (LiveSessionRoom, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+liveRoomColumns+` FROM live_session_rooms WHERE id=$1`, roomID)
	room, err := scanLiveRoom(row)
	if errors.Is(err, sql.ErrNoRows) {
		return room, ErrNotFound
	}
	return room, err
}

// ListLiveRoomsForSession lists every collaborative-eligible item in the
// session's list alongside its room (if one has been created yet) and live
// presence count.
func (s *Store) ListLiveRoomsForSession(ctx context.Context, sessionID string) ([]LiveRoomSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT li.id, li.title, c.title, r.id, r.status,
		        COALESCE((SELECT count(*) FROM live_room_participants p WHERE p.room_id=r.id AND p.left_at IS NULL), 0)
		 FROM live_sessions s
		 JOIN list_items li ON li.list_id = s.problem_list_id
		 LEFT JOIN challenges c ON c.id = li.linked_challenge_id
		 LEFT JOIN live_session_rooms r ON r.session_id = s.id AND r.list_item_id = li.id
		 WHERE s.id=$1
		 ORDER BY li.ordinal`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LiveRoomSummary{}
	for rows.Next() {
		var x LiveRoomSummary
		if err := rows.Scan(&x.ListItemID, &x.ItemTitle, &x.ChallengeTitle, &x.RoomID, &x.Status, &x.PresentCount); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) LiveRoomParticipants(ctx context.Context, roomID string) ([]LiveParticipant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.user_id,u.email,p.joined_at FROM live_room_participants p JOIN users u ON u.id=p.user_id
		 WHERE p.room_id=$1 AND p.left_at IS NULL ORDER BY p.joined_at`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LiveParticipant{}
	for rows.Next() {
		var p LiveParticipant
		if err := rows.Scan(&p.UserID, &p.Email, &p.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// JoinLiveRoom always inserts a fresh presence row — re-entering after a
// left_at is a new row, so back-and-forth history is preserved as-is.
func (s *Store) JoinLiveRoom(ctx context.Context, roomID, userID string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `INSERT INTO live_room_participants(room_id,user_id) VALUES($1,$2) RETURNING id`, roomID, userID).Scan(&id)
	return id, err
}

func (s *Store) LeaveLiveRoom(ctx context.Context, participantID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE live_room_participants SET left_at=now() WHERE id=$1 AND left_at IS NULL`, participantID)
	return err
}

// MarkLiveRoomDone flags the participant as done and reports whether every
// participant currently present has now marked done.
func (s *Store) MarkLiveRoomDone(ctx context.Context, roomID, userID string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `UPDATE live_room_participants SET marked_done=true WHERE room_id=$1 AND user_id=$2 AND left_at IS NULL`, roomID, userID)
	if err != nil {
		return false, err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return false, ErrNotFound
	}
	var allDone bool
	err = tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM live_room_participants WHERE room_id=$1 AND left_at IS NULL)
		 AND NOT EXISTS(SELECT 1 FROM live_room_participants WHERE room_id=$1 AND left_at IS NULL AND marked_done=false)`,
		roomID).Scan(&allDone)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return allDone, nil
}

func (s *Store) SaveLiveRoomDocSnapshot(ctx context.Context, roomID string, doc []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE live_session_rooms SET doc_snapshot=$2, snapshot_updated_at=now() WHERE id=$1 AND status='open'`, roomID, doc)
	return err
}

func (s *Store) SaveLiveRoomTextSnapshot(ctx context.Context, roomID, text string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE live_session_rooms SET text_snapshot=$2, snapshot_updated_at=now() WHERE id=$1 AND status='open'`, roomID, text)
	return err
}

func (s *Store) GetLiveRoomDocSnapshot(ctx context.Context, roomID string) ([]byte, error) {
	var doc []byte
	err := s.db.QueryRowContext(ctx, `SELECT doc_snapshot FROM live_session_rooms WHERE id=$1`, roomID).Scan(&doc)
	return doc, err
}

// FreezeLiveRoom is a no-op (ErrConflict) once the room is already frozen,
// so every trigger can call it unconditionally.
func (s *Store) FreezeLiveRoom(ctx context.Context, roomID, reason string) (LiveSessionRoom, error) {
	row := s.db.QueryRowContext(ctx,
		`UPDATE live_session_rooms SET status='frozen', frozen_at=now(), frozen_reason=$2
		 WHERE id=$1 AND status='open' RETURNING `+liveRoomColumns, roomID, reason)
	room, err := scanLiveRoom(row)
	if errors.Is(err, sql.ErrNoRows) {
		return LiveSessionRoom{}, ErrConflict
	}
	return room, err
}

func (s *Store) FreezeOpenLiveRoomsForSession(ctx context.Context, sessionID, reason string) ([]LiveSessionRoom, error) {
	rows, err := s.db.QueryContext(ctx,
		`UPDATE live_session_rooms SET status='frozen', frozen_at=now(), frozen_reason=$2
		 WHERE session_id=$1 AND status='open' RETURNING `+liveRoomColumns, sessionID, reason)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LiveSessionRoom{}
	for rows.Next() {
		r, err := scanLiveRoom(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LiveRoomChallenge resolves the judge inputs for a list item: the linked
// challenge (empty if the item has no gabarito) and the language to submit
// as (the challenge's fixed language, defaulting to python when the
// challenge accepts any language).
func (s *Store) LiveRoomChallenge(ctx context.Context, listItemID string) (challengeID, language string, err error) {
	var cid, lang sql.NullString
	err = s.db.QueryRowContext(ctx,
		`SELECT li.linked_challenge_id, c.language FROM list_items li LEFT JOIN challenges c ON c.id = li.linked_challenge_id WHERE li.id = $1`,
		listItemID).Scan(&cid, &lang)
	if err != nil {
		return "", "", err
	}
	if !cid.Valid {
		return "", "", nil
	}
	language = lang.String
	if language == "" {
		language = "python"
	}
	return cid.String, language, nil
}

// PublishLiveRoomFrame relays an opaque room frame (update or hydration
// bytes) to every API instance's websocket clients for that room.
func (s *Store) PublishLiveRoomFrame(ctx context.Context, roomID string, payload []byte) error {
	if s.rdb == nil {
		return nil
	}
	envelope, err := json.Marshal(map[string]string{"room_id": roomID, "data": base64.StdEncoding.EncodeToString(payload)})
	if err != nil {
		return err
	}
	return s.rdb.Publish(ctx, LiveRoomEventsChannel, envelope).Err()
}

// PublishLiveRoomClose asks every API instance to drop its websocket
// connections for a frozen room.
func (s *Store) PublishLiveRoomClose(ctx context.Context, roomID string) error {
	if s.rdb == nil {
		return nil
	}
	envelope, err := json.Marshal(map[string]string{"room_id": roomID, "close": "1"})
	if err != nil {
		return err
	}
	return s.rdb.Publish(ctx, LiveRoomEventsChannel, envelope).Err()
}

func (s *Store) LiveRoomResultSubmission(ctx context.Context, roomID string) (Submission, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT s.id, s.challenge_id, s.language, s.source_code, s.status,
		        s.exec_time_ms, s.memory_peak_mb, s.stderr, s.stdout, s.expected_output,
		        s.failed_input, s.failed_is_sample,
		        s.passed_count, s.total_test_cases,
		        c.time_limit_ms, c.memory_limit_mb,
		        s.created_at, s.updated_at, s.live_session_id
		 FROM submissions s JOIN challenges c ON c.id = s.challenge_id
		 WHERE s.live_room_id = $1 ORDER BY s.created_at DESC LIMIT 1`, roomID)
	var sub Submission
	if err := row.Scan(&sub.ID, &sub.ChallengeID, &sub.Language, &sub.SourceCode, &sub.Status,
		&sub.ExecTimeMs, &sub.MemoryPeakMb, &sub.Stderr, &sub.Stdout, &sub.ExpectedOutput,
		&sub.FailedInput, &sub.FailedIsSample,
		&sub.PassedCount, &sub.TotalTestCases,
		&sub.TimeLimitMs, &sub.MemoryLimitMb,
		&sub.CreatedAt, &sub.UpdatedAt, &sub.LiveSessionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Submission{}, ErrNotFound
		}
		return Submission{}, err
	}
	return sub, nil
}
