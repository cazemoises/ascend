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
	// nil for a teacher, since it isn't their completion to report.
	Completed *bool     `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	Title      string
	Difficulty string
	IsBonus    bool
	Body       string
}

type UpdateListItemRequest struct {
	Title      string
	Difficulty string
	IsBonus    bool
	Body       string
}

// ReorderItem is one row of a PATCH /lists/:id/reorder batch.
type ReorderItem struct {
	ItemID  string
	Ordinal int
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
// published); anyone else sees only published lists, platform-wide. Newest
// week first; lists with no week_start sort last, by created_at among
// themselves.
func (s *Store) ListProblemListsForViewer(ctx context.Context, viewerID, role string) ([]ProblemList, error) {
	const order = ` ORDER BY week_start DESC NULLS LAST, created_at DESC`
	var rows *sql.Rows
	var err error
	if role == "teacher" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+problemListColumns+` FROM problem_lists WHERE teacher_id = $1`+order,
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
			`SELECT id, list_id, ordinal, title, difficulty, is_bonus, body, created_at, updated_at
			 FROM list_items WHERE list_id = $1 ORDER BY ordinal`, listID)
		if err != nil {
			return nil, fmt.Errorf("list items: %w", err)
		}
		defer rows.Close()

		items := make([]ListItem, 0)
		for rows.Next() {
			var it ListItem
			if err := rows.Scan(&it.ID, &it.ListID, &it.Ordinal, &it.Title, &it.Difficulty,
				&it.IsBonus, &it.Body, &it.CreatedAt, &it.UpdatedAt); err != nil {
				return nil, err
			}
			items = append(items, it)
		}
		return items, rows.Err()
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT li.id, li.list_id, li.ordinal, li.title, li.difficulty, li.is_bonus, li.body,
		        li.created_at, li.updated_at, (lic.id IS NOT NULL) AS completed
		 FROM list_items li
		 LEFT JOIN list_item_completions lic
		   ON lic.list_item_id = li.id AND lic.student_id = $2
		 WHERE li.list_id = $1
		 ORDER BY li.ordinal`, listID, viewerID)
	if err != nil {
		return nil, fmt.Errorf("list items with completion: %w", err)
	}
	defer rows.Close()

	items := make([]ListItem, 0)
	for rows.Next() {
		var it ListItem
		var completed bool
		if err := rows.Scan(&it.ID, &it.ListID, &it.Ordinal, &it.Title, &it.Difficulty,
			&it.IsBonus, &it.Body, &it.CreatedAt, &it.UpdatedAt, &completed); err != nil {
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

	var it ListItem
	err = tx.QueryRowContext(ctx,
		`INSERT INTO list_items (list_id, ordinal, title, difficulty, is_bonus, body)
		 VALUES ($1, (SELECT COALESCE(MAX(ordinal), -1) + 1 FROM list_items WHERE list_id = $1), $2, $3, $4, $5)
		 RETURNING id, list_id, ordinal, title, difficulty, is_bonus, body, created_at, updated_at`,
		listID, req.Title, req.Difficulty, req.IsBonus, req.Body,
	).Scan(&it.ID, &it.ListID, &it.Ordinal, &it.Title, &it.Difficulty, &it.IsBonus, &it.Body,
		&it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return ListItem{}, fmt.Errorf("insert list item: %w", err)
	}
	return it, tx.Commit()
}

// UpdateListItem and DeleteListItem join through problem_lists to scope the
// mutation to the requesting teacher's own items.
func (s *Store) UpdateListItem(ctx context.Context, itemID, teacherID string, req UpdateListItemRequest) (ListItem, error) {
	row := s.db.QueryRowContext(ctx,
		`UPDATE list_items li
		 SET title = $3, difficulty = $4, is_bonus = $5, body = $6, updated_at = now()
		 FROM problem_lists pl
		 WHERE li.id = $1 AND li.list_id = pl.id AND pl.teacher_id = $2
		 RETURNING li.id, li.list_id, li.ordinal, li.title, li.difficulty, li.is_bonus, li.body,
		           li.created_at, li.updated_at`,
		itemID, teacherID, req.Title, req.Difficulty, req.IsBonus, req.Body)

	var it ListItem
	err := row.Scan(&it.ID, &it.ListID, &it.Ordinal, &it.Title, &it.Difficulty, &it.IsBonus, &it.Body,
		&it.CreatedAt, &it.UpdatedAt)
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
