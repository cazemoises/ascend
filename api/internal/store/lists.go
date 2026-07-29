package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type ProblemList struct {
	ID          string     `json:"id"`
	TeacherID   string     `json:"teacher_id"`
	Title       string     `json:"title"`
	WeekLabel   *string    `json:"week_label"`
	WeekStart   *time.Time `json:"week_start"`
	WeekEnd     *time.Time `json:"week_end"`
	Description *string    `json:"description"`
	Published   bool       `json:"published"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ListItem struct {
	ID         string `json:"id"`
	ListID     string `json:"list_id"`
	Ordinal    int    `json:"ordinal"`
	Title      string `json:"title"`
	Difficulty string `json:"difficulty"`
	IsBonus    bool   `json:"is_bonus"`
	Body       string `json:"body"`
	// Completed is only populated for a student viewer (GetProblemListDetail);
	// nil for a teacher, since it isn't their completion to report. For an
	// item with LinkedChallengeID set, it's derived from the challenge
	// "solved" rule instead of list_item_completions.
	Completed *bool `json:"completed"`
	// LinkedChallengeID, when set, means this item's completion is derived
	// automatically from the student having solved that challenge, not a
	// self-declared checkbox. ChallengeTitle/ChallengeSlug are populated
	// alongside it in GetProblemListDetail (nil elsewhere) so the frontend
	// can link to /challenges/:slug without a second request.
	LinkedChallengeID *string   `json:"linked_challenge_id"`
	ChallengeTitle    *string   `json:"challenge_title"`
	ChallengeSlug     *string   `json:"challenge_slug"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ProblemListDetail struct {
	ProblemList
	Items []ListItem `json:"items"`
}

type CreateProblemListRequest struct {
	TeacherID   string
	Title       string
	WeekLabel   *string
	WeekStart   *time.Time
	WeekEnd     *time.Time
	Description *string
}

type UpdateProblemListRequest struct {
	Title       string
	WeekLabel   *string
	WeekStart   *time.Time
	WeekEnd     *time.Time
	Description *string
	Published   bool
}

type CreateListItemRequest struct {
	Title             string
	Difficulty        string
	IsBonus           bool
	Body              string
	LinkedChallengeID *string
	// LinkedChallengeSlug is only meaningful for ImportProblemList: the
	// teacher writing an import JSON by hand knows a challenge's slug, not
	// its UUID, so ImportProblemList resolves it to LinkedChallengeID
	// before inserting. Ignored by CreateListItem/UpdateListItem, which
	// only ever receive LinkedChallengeID directly (from the studio
	// dropdown, which already has the real id).
	LinkedChallengeSlug *string
}

type UpdateListItemRequest struct {
	Title             string
	Difficulty        string
	IsBonus           bool
	Body              string
	LinkedChallengeID *string
}

// ReorderItem is one row of a PATCH /lists/:id/reorder batch.
type ReorderItem struct {
	ItemID  string
	Ordinal int
}

// ImportProblemListRequest is the payload for POST /lists/import — a list
// plus every item to create with it, in one call.
type ImportProblemListRequest struct {
	TeacherID   string
	Title       string
	WeekLabel   *string
	WeekStart   *time.Time
	WeekEnd     *time.Time
	Description *string
	Items       []CreateListItemRequest
}

const problemListColumns = `id, teacher_id, title, week_label, description, published, created_at, updated_at, week_start, week_end`

func scanProblemList(row interface {
	Scan(...any) error
}) (ProblemList, error) {
	var pl ProblemList
	err := row.Scan(&pl.ID, &pl.TeacherID, &pl.Title, &pl.WeekLabel, &pl.Description,
		&pl.Published, &pl.CreatedAt, &pl.UpdatedAt, &pl.WeekStart, &pl.WeekEnd)
	return pl, err
}

func (s *Store) CreateProblemList(ctx context.Context, req CreateProblemListRequest) (ProblemList, error) {
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO problem_lists (teacher_id, title, week_label, description, week_start, week_end)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+problemListColumns,
		req.TeacherID, req.Title, req.WeekLabel, req.Description, req.WeekStart, req.WeekEnd)
	pl, err := scanProblemList(row)
	if err != nil {
		return ProblemList{}, fmt.Errorf("create problem list: %w", err)
	}
	return pl, nil
}

// ListProblemListsForViewer applies the same visibility rule everywhere a
// list feed is served: a teacher sees every list they own (draft and
// published) plus every published list platform-wide, including other
// teachers'; anyone else sees only published lists, platform-wide. Newest
// week first; lists with no week_start sort last, by created_at among
// themselves.
func (s *Store) ListProblemListsForViewer(ctx context.Context, viewerID, role string) ([]ProblemList, error) {
	const order = ` ORDER BY week_start DESC NULLS LAST, created_at DESC`
	var rows *sql.Rows
	var err error
	if role == "teacher" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+problemListColumns+` FROM problem_lists WHERE teacher_id = $1 OR published = true`+order,
			viewerID)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+problemListColumns+` FROM problem_lists WHERE published = true`+order)
	}
	if err != nil {
		return nil, fmt.Errorf("list problem lists: %w", err)
	}
	defer rows.Close()

	lists := make([]ProblemList, 0)
	for rows.Next() {
		pl, err := scanProblemList(rows)
		if err != nil {
			return nil, err
		}
		lists = append(lists, pl)
	}
	return lists, rows.Err()
}

// GetProblemListDetail returns a list and its items ordered by ordinal. A
// draft list (published=false) is visible only to its owning teacher — a
// student, or any other teacher, gets ErrNotFound rather than a 403, so
// drafts don't leak existence to non-owners. For a student viewer, each item
// carries their own completed flag (never another student's).
func (s *Store) GetProblemListDetail(ctx context.Context, listID, viewerID, role string) (ProblemListDetail, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+problemListColumns+` FROM problem_lists WHERE id = $1`, listID)
	pl, err := scanProblemList(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ProblemListDetail{}, ErrNotFound
	}
	if err != nil {
		return ProblemListDetail{}, fmt.Errorf("get problem list: %w", err)
	}
	if !pl.Published && pl.TeacherID != viewerID {
		return ProblemListDetail{}, ErrNotFound
	}

	items, err := s.listItemsForDetail(ctx, listID, viewerID, role)
	if err != nil {
		return ProblemListDetail{}, err
	}

	return ProblemListDetail{ProblemList: pl, Items: items}, nil
}

func (s *Store) listItemsForDetail(ctx context.Context, listID, viewerID, role string) ([]ListItem, error) {
	if role != "student" {
		rows, err := s.db.QueryContext(ctx,
			`SELECT li.id, li.list_id, li.ordinal, li.title, li.difficulty, li.is_bonus, li.body,
			        li.created_at, li.updated_at, li.linked_challenge_id, c.title, c.slug
			 FROM list_items li
			 LEFT JOIN challenges c ON c.id = li.linked_challenge_id
			 WHERE li.list_id = $1
			 ORDER BY li.ordinal`, listID)
		if err != nil {
			return nil, fmt.Errorf("list items: %w", err)
		}
		defer rows.Close()

		items := make([]ListItem, 0)
		for rows.Next() {
			var it ListItem
			if err := rows.Scan(&it.ID, &it.ListID, &it.Ordinal, &it.Title, &it.Difficulty,
				&it.IsBonus, &it.Body, &it.CreatedAt, &it.UpdatedAt,
				&it.LinkedChallengeID, &it.ChallengeTitle, &it.ChallengeSlug); err != nil {
				return nil, err
			}
			items = append(items, it)
		}
		return items, rows.Err()
	}

	// completed: for a linked item, derived from the same "solved" rule
	// challenges use (accepted, non-test-run submission) instead of
	// list_item_completions — an item can't have both, so exactly one side
	// of the CASE is ever meaningful for a given row. viewerID is passed
	// twice (as $2 and $3): sharing one placeholder between the plain uuid
	// comparison below and solvedExpr's NULLIF($N, '')::uuid cast makes
	// Postgres unify both usages to the same type, which fails to parse the
	// '' literal as a uuid — separate placeholders sidestep that entirely.
	rows, err := s.db.QueryContext(ctx,
		`SELECT li.id, li.list_id, li.ordinal, li.title, li.difficulty, li.is_bonus, li.body,
		        li.created_at, li.updated_at, li.linked_challenge_id, c.title, c.slug,
		        CASE WHEN li.linked_challenge_id IS NOT NULL
		             THEN `+solvedExpr("li.linked_challenge_id", 3)+`
		             ELSE (lic.id IS NOT NULL)
		        END AS completed
		 FROM list_items li
		 LEFT JOIN list_item_completions lic
		   ON lic.list_item_id = li.id AND lic.student_id = $2
		 LEFT JOIN challenges c ON c.id = li.linked_challenge_id
		 WHERE li.list_id = $1
		 ORDER BY li.ordinal`, listID, viewerID, viewerID)
	if err != nil {
		return nil, fmt.Errorf("list items with completion: %w", err)
	}
	defer rows.Close()

	items := make([]ListItem, 0)
	for rows.Next() {
		var it ListItem
		var completed bool
		if err := rows.Scan(&it.ID, &it.ListID, &it.Ordinal, &it.Title, &it.Difficulty,
			&it.IsBonus, &it.Body, &it.CreatedAt, &it.UpdatedAt,
			&it.LinkedChallengeID, &it.ChallengeTitle, &it.ChallengeSlug, &completed); err != nil {
			return nil, err
		}
		it.Completed = &completed
		items = append(items, it)
	}
	return items, rows.Err()
}

// UpdateProblemList and DeleteProblemList are ownership-scoped in the query
// itself (WHERE ... AND teacher_id = $teacherID): a non-owner's request
// matches zero rows and gets ErrNotFound, same as a nonexistent list.
func (s *Store) UpdateProblemList(ctx context.Context, listID, teacherID string, req UpdateProblemListRequest) (ProblemList, error) {
	row := s.db.QueryRowContext(ctx,
		`UPDATE problem_lists
		 SET title = $3, week_label = $4, description = $5, published = $6, week_start = $7, week_end = $8, updated_at = now()
		 WHERE id = $1 AND teacher_id = $2
		 RETURNING `+problemListColumns,
		listID, teacherID, req.Title, req.WeekLabel, req.Description, req.Published, req.WeekStart, req.WeekEnd)
	pl, err := scanProblemList(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ProblemList{}, ErrNotFound
	}
	if err != nil {
		return ProblemList{}, fmt.Errorf("update problem list: %w", err)
	}
	return pl, nil
}

func (s *Store) DeleteProblemList(ctx context.Context, listID, teacherID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM problem_lists WHERE id = $1 AND teacher_id = $2`, listID, teacherID)
	if err != nil {
		return fmt.Errorf("delete problem list: %w", err)
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

// ErrLinkedChallengeNotFound means a list item's linked_challenge_id doesn't
// reference an existing challenge. Challenges have no per-teacher owner in
// this system (any teacher can already edit/delete any challenge), so this
// only checks existence, not ownership.
var ErrLinkedChallengeNotFound = errors.New("linked challenge not found")

// ImportChallengeSlugNotFoundError means an import item's
// linked_challenge_slug doesn't match any challenge — identifies which item
// (by index) and which slug, so the caller can report it precisely.
type ImportChallengeSlugNotFoundError struct {
	Index int
	Slug  string
}

func (e *ImportChallengeSlugNotFoundError) Error() string {
	return fmt.Sprintf("item %d: linked_challenge_slug %q not found", e.Index, e.Slug)
}

// CreateListItem verifies the caller owns the target list before inserting,
// inside one transaction so the ownership check and the ordinal-assignment
// insert see a consistent snapshot.
func (s *Store) CreateListItem(ctx context.Context, listID, teacherID string, req CreateListItemRequest) (ListItem, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ListItem{}, err
	}
	defer tx.Rollback()

	var owns bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM problem_lists WHERE id = $1 AND teacher_id = $2)`,
		listID, teacherID).Scan(&owns); err != nil {
		return ListItem{}, fmt.Errorf("check list ownership: %w", err)
	}
	if !owns {
		return ListItem{}, ErrNotFound
	}

	if req.LinkedChallengeID != nil {
		var exists bool
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM challenges WHERE id = $1)`, *req.LinkedChallengeID,
		).Scan(&exists); err != nil {
			return ListItem{}, fmt.Errorf("check linked challenge: %w", err)
		}
		if !exists {
			return ListItem{}, ErrLinkedChallengeNotFound
		}
	}

	var it ListItem
	err = tx.QueryRowContext(ctx,
		`INSERT INTO list_items (list_id, ordinal, title, difficulty, is_bonus, body, linked_challenge_id)
		 VALUES ($1, (SELECT COALESCE(MAX(ordinal), -1) + 1 FROM list_items WHERE list_id = $1), $2, $3, $4, $5, $6)
		 RETURNING id, list_id, ordinal, title, difficulty, is_bonus, body, created_at, updated_at, linked_challenge_id`,
		listID, req.Title, req.Difficulty, req.IsBonus, req.Body, req.LinkedChallengeID,
	).Scan(&it.ID, &it.ListID, &it.Ordinal, &it.Title, &it.Difficulty, &it.IsBonus, &it.Body,
		&it.CreatedAt, &it.UpdatedAt, &it.LinkedChallengeID)
	if err != nil {
		return ListItem{}, fmt.Errorf("insert list item: %w", err)
	}
	return it, tx.Commit()
}

// ImportProblemList creates a list and all of its items in a single
// transaction: an insert failure on any item (or the list itself) rolls back
// the whole request, so a bad payload never leaves an orphaned list behind.
// Ordinals are assigned by array position, same as a fresh CreateListItem
// sequence would produce.
func (s *Store) ImportProblemList(ctx context.Context, req ImportProblemListRequest) (ProblemListDetail, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProblemListDetail{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx,
		`INSERT INTO problem_lists (teacher_id, title, week_label, description, week_start, week_end)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+problemListColumns,
		req.TeacherID, req.Title, req.WeekLabel, req.Description, req.WeekStart, req.WeekEnd)
	pl, err := scanProblemList(row)
	if err != nil {
		return ProblemListDetail{}, fmt.Errorf("insert problem list: %w", err)
	}

	items := make([]ListItem, 0, len(req.Items))
	for i, it := range req.Items {
		// Same resolution CreateListItem applies to a direct id, plus the
		// slug lookup import needs — a teacher writing JSON by hand knows a
		// challenge's slug, not its UUID.
		linkedChallengeID := it.LinkedChallengeID
		if linkedChallengeID == nil && it.LinkedChallengeSlug != nil && *it.LinkedChallengeSlug != "" {
			var resolvedID string
			err := tx.QueryRowContext(ctx,
				`SELECT id FROM challenges WHERE slug = $1`, *it.LinkedChallengeSlug,
			).Scan(&resolvedID)
			if errors.Is(err, sql.ErrNoRows) {
				return ProblemListDetail{}, &ImportChallengeSlugNotFoundError{Index: i, Slug: *it.LinkedChallengeSlug}
			}
			if err != nil {
				return ProblemListDetail{}, fmt.Errorf("resolve linked_challenge_slug for item %d: %w", i, err)
			}
			linkedChallengeID = &resolvedID
		} else if linkedChallengeID != nil {
			var exists bool
			if err := tx.QueryRowContext(ctx,
				`SELECT EXISTS(SELECT 1 FROM challenges WHERE id = $1)`, *linkedChallengeID,
			).Scan(&exists); err != nil {
				return ProblemListDetail{}, fmt.Errorf("check linked challenge for item %d: %w", i, err)
			}
			if !exists {
				return ProblemListDetail{}, ErrLinkedChallengeNotFound
			}
		}

		var li ListItem
		err := tx.QueryRowContext(ctx,
			`INSERT INTO list_items (list_id, ordinal, title, difficulty, is_bonus, body, linked_challenge_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id, list_id, ordinal, title, difficulty, is_bonus, body, created_at, updated_at, linked_challenge_id`,
			pl.ID, i, it.Title, it.Difficulty, it.IsBonus, it.Body, linkedChallengeID,
		).Scan(&li.ID, &li.ListID, &li.Ordinal, &li.Title, &li.Difficulty, &li.IsBonus, &li.Body,
			&li.CreatedAt, &li.UpdatedAt, &li.LinkedChallengeID)
		if err != nil {
			return ProblemListDetail{}, fmt.Errorf("insert list item %d: %w", i, err)
		}
		items = append(items, li)
	}

	if err := tx.Commit(); err != nil {
		return ProblemListDetail{}, err
	}
	return ProblemListDetail{ProblemList: pl, Items: items}, nil
}

// UpdateListItem and DeleteListItem join through problem_lists to scope the
// mutation to the requesting teacher's own items.
func (s *Store) UpdateListItem(ctx context.Context, itemID, teacherID string, req UpdateListItemRequest) (ListItem, error) {
	if req.LinkedChallengeID != nil {
		var exists bool
		if err := s.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM challenges WHERE id = $1)`, *req.LinkedChallengeID,
		).Scan(&exists); err != nil {
			return ListItem{}, fmt.Errorf("check linked challenge: %w", err)
		}
		if !exists {
			return ListItem{}, ErrLinkedChallengeNotFound
		}
	}

	row := s.db.QueryRowContext(ctx,
		`UPDATE list_items li
		 SET title = $3, difficulty = $4, is_bonus = $5, body = $6, linked_challenge_id = $7, updated_at = now()
		 FROM problem_lists pl
		 WHERE li.id = $1 AND li.list_id = pl.id AND pl.teacher_id = $2
		 RETURNING li.id, li.list_id, li.ordinal, li.title, li.difficulty, li.is_bonus, li.body,
		           li.created_at, li.updated_at, li.linked_challenge_id`,
		itemID, teacherID, req.Title, req.Difficulty, req.IsBonus, req.Body, req.LinkedChallengeID)

	var it ListItem
	err := row.Scan(&it.ID, &it.ListID, &it.Ordinal, &it.Title, &it.Difficulty, &it.IsBonus, &it.Body,
		&it.CreatedAt, &it.UpdatedAt, &it.LinkedChallengeID)
	if errors.Is(err, sql.ErrNoRows) {
		return ListItem{}, ErrNotFound
	}
	if err != nil {
		return ListItem{}, fmt.Errorf("update list item: %w", err)
	}
	return it, nil
}

func (s *Store) DeleteListItem(ctx context.Context, itemID, teacherID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM list_items li
		 USING problem_lists pl
		 WHERE li.id = $1 AND li.list_id = pl.id AND pl.teacher_id = $2`,
		itemID, teacherID)
	if err != nil {
		return fmt.Errorf("delete list item: %w", err)
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

// ReorderListItems applies a batch of ordinal updates atomically: the list
// ownership check and every row update share one transaction, so a bad item
// id (or one belonging to a different list) rolls back the entire batch
// instead of leaving it half-reordered.
func (s *Store) ReorderListItems(ctx context.Context, listID, teacherID string, items []ReorderItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var owns bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM problem_lists WHERE id = $1 AND teacher_id = $2)`,
		listID, teacherID).Scan(&owns); err != nil {
		return fmt.Errorf("check list ownership: %w", err)
	}
	if !owns {
		return ErrNotFound
	}

	for _, item := range items {
		res, err := tx.ExecContext(ctx,
			`UPDATE list_items SET ordinal = $1, updated_at = now() WHERE id = $2 AND list_id = $3`,
			item.Ordinal, item.ItemID, listID)
		if err != nil {
			return fmt.Errorf("update ordinal for item %s: %w", item.ItemID, err)
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
