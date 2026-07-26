//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/caze/ascend/api/internal/store"
)

func TestCompleteListItem_Idempotent(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "completions-idempotent-teacher@example.com")
	student := createTestUser(t, s, db, ctx, "completions-idempotent-student@example.com")

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Idempotent"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })
	if _, err := s.UpdateProblemList(ctx, list.ID, teacher.ID, store.UpdateProblemListRequest{Title: list.Title, Published: true}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{Title: "Item", Difficulty: "easy", Body: "b"})
	if err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}

	if err := s.CompleteListItem(ctx, item.ID, student.ID); err != nil {
		t.Fatalf("CompleteListItem (1st call): %v", err)
	}
	if err := s.CompleteListItem(ctx, item.ID, student.ID); err != nil {
		t.Fatalf("CompleteListItem (2nd call): %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM list_item_completions WHERE list_item_id = $1 AND student_id = $2`,
		item.ID, student.ID).Scan(&count); err != nil {
		t.Fatalf("count completions: %v", err)
	}
	if count != 1 {
		t.Errorf("completion rows = %d, want 1 (calling complete twice must not duplicate)", count)
	}
}

func TestCompleteListItem_DraftListNotFound(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "completions-draft-teacher@example.com")
	student := createTestUser(t, s, db, ctx, "completions-draft-student@example.com")

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Still Draft"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })

	item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{Title: "Item", Difficulty: "easy", Body: "b"})
	if err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}

	if err := s.CompleteListItem(ctx, item.ID, student.ID); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound completing an item on a draft list, got %v", err)
	}
}

func TestUncompleteListItem_IdempotentWhenNeverCompleted(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "completions-uncomplete-teacher@example.com")
	student := createTestUser(t, s, db, ctx, "completions-uncomplete-student@example.com")

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Uncomplete Test"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })
	if _, err := s.UpdateProblemList(ctx, list.ID, teacher.ID, store.UpdateProblemListRequest{Title: list.Title, Published: true}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{Title: "Item", Difficulty: "easy", Body: "b"})
	if err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}

	if err := s.UncompleteListItem(ctx, item.ID, student.ID); err != nil {
		t.Errorf("uncomplete on a never-completed item should be a no-op, got %v", err)
	}
}

// TestGetProblemListDetail_CompletionIsPerStudent is the direct check for
// "a student only ever sees their own completions, never another student's":
// two students, one completes the item, the other's view must show false.
func TestGetProblemListDetail_CompletionIsPerStudent(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "completions-per-student-teacher@example.com")
	studentA := createTestUser(t, s, db, ctx, "completions-per-student-a@example.com")
	studentB := createTestUser(t, s, db, ctx, "completions-per-student-b@example.com")

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Per Student"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })
	if _, err := s.UpdateProblemList(ctx, list.ID, teacher.ID, store.UpdateProblemListRequest{Title: list.Title, Published: true}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{Title: "Item", Difficulty: "easy", Body: "b"})
	if err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}

	if err := s.CompleteListItem(ctx, item.ID, studentA.ID); err != nil {
		t.Fatalf("CompleteListItem for studentA: %v", err)
	}

	detailA, err := s.GetProblemListDetail(ctx, list.ID, studentA.ID, "student")
	if err != nil {
		t.Fatalf("GetProblemListDetail (studentA): %v", err)
	}
	if detailA.Items[0].Completed == nil || !*detailA.Items[0].Completed {
		t.Error("studentA completed the item — their view must show completed=true")
	}

	detailB, err := s.GetProblemListDetail(ctx, list.ID, studentB.ID, "student")
	if err != nil {
		t.Fatalf("GetProblemListDetail (studentB): %v", err)
	}
	if detailB.Items[0].Completed == nil || *detailB.Items[0].Completed {
		t.Error("studentB never completed the item — their view must show completed=false, not studentA's")
	}
}
