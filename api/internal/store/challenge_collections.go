package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ChallengeCollection groups challenges by a named theme/cycle — order
// only, no dates (unlike ProblemList's week-based grouping). GroupID
// optionally nests it under a ChallengeGroup; nil means it sits at the
// top level alongside groups (see challenge_groups.go).
type ChallengeCollection struct {
	ID        string    `json:"id"`
	TeacherID string    `json:"teacher_id"`
	Title     string    `json:"title"`
	Ordinal   int       `json:"ordinal"`
	GroupID   *string   `json:"group_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateChallengeCollectionRequest struct {
	TeacherID string
	Title     string
	// Ordinal nil means "next available among this teacher's collections".
	Ordinal *int
	// GroupID nil means uncategorized (no parent group); non-nil must
	// reference a group owned by the same teacher.
	GroupID *string
}

type UpdateChallengeCollectionRequest struct {
	Title   string
	Ordinal int
	GroupID *string
}

// ErrCollectionNotFound means a challenge's collection_id doesn't reference
// an existing challenge_collections row.
var ErrCollectionNotFound = errors.New("challenge collection not found")

// queryRower is satisfied by both *sql.DB and *sql.Tx, so
// validateCollectionID works the same whether the caller is already inside
// a transaction (ImportChallenges) or not (CreateChallenge/UpdateChallenge).
type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// validateCollectionID checks collectionID exists (a nil pointer, meaning
// "uncategorized", always passes). Challenges have no per-teacher owner in
// this system, so — like linked_challenge_id on list_items — this only
// checks existence, not ownership.
func validateCollectionID(ctx context.Context, q queryRower, collectionID *string) error {
	if collectionID == nil {
		return nil
	}
	var exists bool
	if err := q.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM challenge_collections WHERE id = $1)`, *collectionID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check challenge collection: %w", err)
	}
	if !exists {
		return ErrCollectionNotFound
	}
	return nil
}

const challengeCollectionColumns = `id, teacher_id, title, ordinal, group_id, created_at, updated_at`

func scanChallengeCollection(row interface {
	Scan(...any) error
}) (ChallengeCollection, error) {
	var cc ChallengeCollection
	err := row.Scan(&cc.ID, &cc.TeacherID, &cc.Title, &cc.Ordinal, &cc.GroupID, &cc.CreatedAt, &cc.UpdatedAt)
	return cc, err
}

func (s *Store) CreateChallengeCollection(ctx context.Context, req CreateChallengeCollectionRequest) (ChallengeCollection, error) {
	if err := validateGroupID(ctx, s.db, req.GroupID, req.TeacherID); err != nil {
		return ChallengeCollection{}, err
	}

	var row *sql.Row
	if req.Ordinal != nil {
		row = s.db.QueryRowContext(ctx,
			`INSERT INTO challenge_collections (teacher_id, title, ordinal, group_id)
			 VALUES ($1, $2, $3, $4)
			 RETURNING `+challengeCollectionColumns,
			req.TeacherID, req.Title, *req.Ordinal, req.GroupID)
	} else {
		row = s.db.QueryRowContext(ctx,
			`INSERT INTO challenge_collections (teacher_id, title, ordinal, group_id)
			 VALUES ($1, $2, (SELECT COALESCE(MAX(ordinal), -1) + 1 FROM challenge_collections WHERE teacher_id = $1), $3)
			 RETURNING `+challengeCollectionColumns,
			req.TeacherID, req.Title, req.GroupID)
	}
	cc, err := scanChallengeCollection(row)
	if err != nil {
		return ChallengeCollection{}, fmt.Errorf("insert challenge collection: %w", err)
	}
	return cc, nil
}

// ListChallengeCollections returns one teacher's own collections, ordinal
// ascending — the source list for the studio's "Vincular a coleção"
// dropdown.
func (s *Store) ListChallengeCollections(ctx context.Context, teacherID string) ([]ChallengeCollection, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+challengeCollectionColumns+` FROM challenge_collections WHERE teacher_id = $1 ORDER BY ordinal ASC`,
		teacherID)
	if err != nil {
		return nil, fmt.Errorf("list challenge collections: %w", err)
	}
	defer rows.Close()

	ccs := make([]ChallengeCollection, 0)
	for rows.Next() {
		cc, err := scanChallengeCollection(rows)
		if err != nil {
			return nil, err
		}
		ccs = append(ccs, cc)
	}
	return ccs, rows.Err()
}

// UpdateChallengeCollection and DeleteChallengeCollection are
// ownership-scoped in the query itself (WHERE ... AND teacher_id =
// $teacherID), same pattern as UpdateProblemList/DeleteProblemList: a
// non-owner's request matches zero rows and gets ErrNotFound.
func (s *Store) UpdateChallengeCollection(ctx context.Context, id, teacherID string, req UpdateChallengeCollectionRequest) (ChallengeCollection, error) {
	if err := validateGroupID(ctx, s.db, req.GroupID, teacherID); err != nil {
		return ChallengeCollection{}, err
	}

	row := s.db.QueryRowContext(ctx,
		`UPDATE challenge_collections SET title = $3, ordinal = $4, group_id = $5, updated_at = now()
		 WHERE id = $1 AND teacher_id = $2
		 RETURNING `+challengeCollectionColumns,
		id, teacherID, req.Title, req.Ordinal, req.GroupID)
	cc, err := scanChallengeCollection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ChallengeCollection{}, ErrNotFound
	}
	if err != nil {
		return ChallengeCollection{}, fmt.Errorf("update challenge collection: %w", err)
	}
	return cc, nil
}

// CollectionReorderItem is one row of a PATCH /challenge-collections/reorder
// batch.
type CollectionReorderItem struct {
	ID      string
	Ordinal int
}

// ReorderChallengeCollections applies a batch of ordinal updates atomically:
// any item that doesn't exist or isn't owned by teacherID rolls back the
// whole batch instead of leaving it half-reordered. Unlike
// ReorderListItems, there's no separate parent entity to check ownership
// of first — collections carry teacher_id directly — so each row's own
// UPDATE ... WHERE id = $1 AND teacher_id = $2 does the ownership check.
func (s *Store) ReorderChallengeCollections(ctx context.Context, teacherID string, items []CollectionReorderItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, item := range items {
		res, err := tx.ExecContext(ctx,
			`UPDATE challenge_collections SET ordinal = $1, updated_at = now() WHERE id = $2 AND teacher_id = $3`,
			item.Ordinal, item.ID, teacherID)
		if err != nil {
			return fmt.Errorf("update ordinal for collection %s: %w", item.ID, err)
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

func (s *Store) DeleteChallengeCollection(ctx context.Context, id, teacherID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM challenge_collections WHERE id = $1 AND teacher_id = $2`, id, teacherID)
	if err != nil {
		return fmt.Errorf("delete challenge collection: %w", err)
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
