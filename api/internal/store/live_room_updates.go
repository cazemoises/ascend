package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type LiveRoomUpdate struct {
	Version uint64
	Payload []byte
}

// AppendLiveRoomUpdate locks the room through the version increment until the
// journal insert commits. Versions therefore follow commit order, with no gaps.
func (s *Store) AppendLiveRoomUpdate(ctx context.Context, roomID string, payload []byte) (uint64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin room update: %w", err)
	}
	defer tx.Rollback()
	var version uint64
	err = tx.QueryRowContext(ctx, `UPDATE live_session_rooms SET doc_version=doc_version+1
 WHERE id=$1 AND status='open' RETURNING doc_version`, roomID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrConflict
	}
	if err != nil {
		return 0, fmt.Errorf("advance room version: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO live_room_updates(room_id,version,payload) VALUES($1,$2,$3)`, roomID, version, payload); err != nil {
		return 0, fmt.Errorf("persist room update: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit room update: %w", err)
	}
	return version, nil
}

// ReadLiveRoomUpdates reads the immutable legacy baseline and journal in one
// PostgreSQL statement snapshot. Never prune using a client-supplied snapshot:
// a version alone cannot prove that it contains all causal Yjs dependencies.
func (s *Store) ReadLiveRoomUpdates(ctx context.Context, roomID string, after int64) ([]LiveRoomUpdate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT version,payload FROM (
 SELECT 0::bigint AS version,doc_baseline AS payload FROM live_session_rooms WHERE id=$1 AND $2<0
 UNION ALL SELECT version,payload FROM live_room_updates WHERE room_id=$1 AND version>$2
 ) updates ORDER BY version`, roomID, after)
	if err != nil {
		return nil, fmt.Errorf("read room journal: %w", err)
	}
	defer rows.Close()
	updates := []LiveRoomUpdate{}
	for rows.Next() {
		var update LiveRoomUpdate
		if err := rows.Scan(&update.Version, &update.Payload); err != nil {
			return nil, fmt.Errorf("scan room update: %w", err)
		}
		updates = append(updates, update)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate room updates: %w", err)
	}
	return updates, nil
}

// SaveLiveRoomSnapshot atomically saves both representations only if the
// client has replayed the current journal frontier. Delayed saves cannot roll
// the cache back, and the authoritative journal is retained independently.
func (s *Store) SaveLiveRoomSnapshot(ctx context.Context, roomID string, version uint64, doc []byte, text string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE live_session_rooms SET doc_snapshot=$3,text_snapshot=$4,
 snapshot_version=$2,snapshot_updated_at=now() WHERE id=$1 AND status='open'
 AND doc_version=$2 AND snapshot_version<=$2`, roomID, version, doc, text)
	if err != nil {
		return false, fmt.Errorf("save versioned room snapshot: %w", err)
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
