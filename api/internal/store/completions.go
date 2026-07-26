package store

import "context"

// CompleteListItem records a student's self-declared completion. Idempotent:
// calling it again is a no-op (ON CONFLICT DO NOTHING against the
// list_item_completions UNIQUE(list_item_id, student_id) constraint), never
// a duplicate row or an error. The item's parent list must be published —
// a student can't complete something on a draft list they can't even see.
func (s *Store) CompleteListItem(ctx context.Context, itemID, studentID string) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO list_item_completions (list_item_id, student_id)
		 SELECT li.id, $2
		 FROM list_items li
		 JOIN problem_lists pl ON pl.id = li.list_id
		 WHERE li.id = $1 AND pl.published = true
		 ON CONFLICT (list_item_id, student_id) DO NOTHING`,
		itemID, studentID)
	if err != nil {
		return err
	}

	// The INSERT ... SELECT matches zero rows both when the item doesn't
	// exist and when its list isn't published — either way the completion
	// wasn't recorded, but RowsAffected also reads 0 on the harmless
	// "already completed" no-op path, so distinguish it with an existence
	// check before reporting ErrNotFound.
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	var eligible bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM list_items li
		   JOIN problem_lists pl ON pl.id = li.list_id
		   WHERE li.id = $1 AND pl.published = true
		 )`, itemID).Scan(&eligible); err != nil {
		return err
	}
	if !eligible {
		return ErrNotFound
	}
	return nil
}

// UncompleteListItem removes a completion, if any. Deleting a row that
// doesn't exist is a no-op, not an error — unmarking twice is fine.
func (s *Store) UncompleteListItem(ctx context.Context, itemID, studentID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM list_item_completions WHERE list_item_id = $1 AND student_id = $2`,
		itemID, studentID)
	return err
}
