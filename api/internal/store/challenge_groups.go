package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ChallengeGroup is the level above ChallengeCollection — e.g. "SQL"
// grouping "SQL Iniciante"/"SQL Avançado" collections together. No
// updated_at column: unlike ChallengeCollection, nothing about a group
// besides title/ordinal ever changes after creation, and neither is
// surfaced anywhere that would need a last-modified timestamp.
type ChallengeGroup struct {
	ID        string    `json:"id"`
	TeacherID string    `json:"teacher_id"`
	Title     string    `json:"title"`
	Ordinal   int       `json:"ordinal"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateChallengeGroupRequest struct {
	TeacherID string
	Title     string
	// Ordinal nil means "next available among this teacher's groups".
	Ordinal *int
}

type UpdateChallengeGroupRequest struct {
	Title   string
	Ordinal int
}

// ErrGroupNotFound means a collection's group_id doesn't reference an
// existing challenge_groups row owned by the same teacher.
var ErrGroupNotFound = errors.New("challenge group not found")

// validateGroupID checks groupID exists AND is owned by teacherID (a nil
// pointer, meaning "no group", always passes). Unlike
// validateCollectionID, this checks ownership too — a collection and the
// group it belongs to must belong to the same teacher.
func validateGroupID(ctx context.Context, q queryRower, groupID *string, teacherID string) error {
	if groupID == nil {
		return nil
	}
	var exists bool
	if err := q.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM challenge_groups WHERE id = $1 AND teacher_id = $2)`,
		*groupID, teacherID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check challenge group: %w", err)
	}
	if !exists {
		return ErrGroupNotFound
	}
	return nil
}

const challengeGroupColumns = `id, teacher_id, title, ordinal, created_at`

func scanChallengeGroup(row interface {
	Scan(...any) error
}) (ChallengeGroup, error) {
	var cg ChallengeGroup
	err := row.Scan(&cg.ID, &cg.TeacherID, &cg.Title, &cg.Ordinal, &cg.CreatedAt)
	return cg, err
}

func (s *Store) CreateChallengeGroup(ctx context.Context, req CreateChallengeGroupRequest) (ChallengeGroup, error) {
	var row *sql.Row
	if req.Ordinal != nil {
		row = s.db.QueryRowContext(ctx,
			`INSERT INTO challenge_groups (teacher_id, title, ordinal)
			 VALUES ($1, $2, $3)
			 RETURNING `+challengeGroupColumns,
			req.TeacherID, req.Title, *req.Ordinal)
	} else {
		row = s.db.QueryRowContext(ctx,
			`INSERT INTO challenge_groups (teacher_id, title, ordinal)
			 VALUES ($1, $2, (SELECT COALESCE(MAX(ordinal), -1) + 1 FROM challenge_groups WHERE teacher_id = $1))
			 RETURNING `+challengeGroupColumns,
			req.TeacherID, req.Title)
	}
	cg, err := scanChallengeGroup(row)
	if err != nil {
		return ChallengeGroup{}, fmt.Errorf("insert challenge group: %w", err)
	}
	return cg, nil
}

// ListChallengeGroups returns one teacher's own groups, ordinal ascending
// — the source list for the studio's "Grupo" dropdown on a collection.
func (s *Store) ListChallengeGroups(ctx context.Context, teacherID string) ([]ChallengeGroup, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+challengeGroupColumns+` FROM challenge_groups WHERE teacher_id = $1 ORDER BY ordinal ASC`,
		teacherID)
	if err != nil {
		return nil, fmt.Errorf("list challenge groups: %w", err)
	}
	defer rows.Close()

	cgs := make([]ChallengeGroup, 0)
	for rows.Next() {
		cg, err := scanChallengeGroup(rows)
		if err != nil {
			return nil, err
		}
		cgs = append(cgs, cg)
	}
	return cgs, rows.Err()
}

// UpdateChallengeGroup and DeleteChallengeGroup are ownership-scoped in the
// query itself, same pattern as UpdateChallengeCollection/DeleteChallengeCollection.
func (s *Store) UpdateChallengeGroup(ctx context.Context, id, teacherID string, req UpdateChallengeGroupRequest) (ChallengeGroup, error) {
	row := s.db.QueryRowContext(ctx,
		`UPDATE challenge_groups SET title = $3, ordinal = $4
		 WHERE id = $1 AND teacher_id = $2
		 RETURNING `+challengeGroupColumns,
		id, teacherID, req.Title, req.Ordinal)
	cg, err := scanChallengeGroup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ChallengeGroup{}, ErrNotFound
	}
	if err != nil {
		return ChallengeGroup{}, fmt.Errorf("update challenge group: %w", err)
	}
	return cg, nil
}

// GroupReorderItem is one row of a PATCH /challenge-groups/reorder batch.
type GroupReorderItem struct {
	ID      string
	Ordinal int
}

// ReorderChallengeGroups applies a batch of ordinal updates atomically,
// mirroring ReorderChallengeCollections.
func (s *Store) ReorderChallengeGroups(ctx context.Context, teacherID string, items []GroupReorderItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, item := range items {
		res, err := tx.ExecContext(ctx,
			`UPDATE challenge_groups SET ordinal = $1 WHERE id = $2 AND teacher_id = $3`,
			item.Ordinal, item.ID, teacherID)
		if err != nil {
			return fmt.Errorf("update ordinal for group %s: %w", item.ID, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
	}

	return tx.Commit()
}

func (s *Store) DeleteChallengeGroup(ctx context.Context, id, teacherID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM challenge_groups WHERE id = $1 AND teacher_id = $2`, id, teacherID)
	if err != nil {
		return fmt.Errorf("delete challenge group: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
