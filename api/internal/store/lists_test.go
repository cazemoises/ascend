//go:build integration

package store_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/caze/ascend/api/internal/store"
)

// createTestUser is a small helper shared by the lists tests: they need real
// teacher/student rows since problem_lists.teacher_id and
// list_item_completions.student_id are FK-constrained.
func createTestUser(t *testing.T, s *store.Store, db *sql.DB, ctx context.Context, email string) store.User {
	t.Helper()
	u, err := s.CreateUser(ctx, email, "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", email, err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, u.ID) })
	return u
}

func TestCreateProblemList_HappyPath(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "lists-happy-path-teacher@example.com")

	weekLabel := "Semana 12"
	desc := "Intro da lista"
	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{
		TeacherID:   teacher.ID,
		Title:       "Recursão",
		WeekLabel:   &weekLabel,
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })

	if list.ID == "" {
		t.Error("expected non-empty list ID")
	}
	if list.Published {
		t.Error("expected published=false on creation")
	}
	if list.TeacherID != teacher.ID {
		t.Errorf("teacher_id = %q, want %q", list.TeacherID, teacher.ID)
	}
}

func TestCreateListItem_OrdinalSequencing(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "lists-ordinal-teacher@example.com")

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{
		TeacherID: teacher.ID,
		Title:     "Ordinal Test",
	})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })

	for i, wantOrdinal := range []int{0, 1, 2} {
		item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
			Title:      "Item",
			Difficulty: "easy",
			Body:       "Body",
		})
		if err != nil {
			t.Fatalf("CreateListItem #%d: %v", i, err)
		}
		if item.Ordinal != wantOrdinal {
			t.Errorf("item #%d ordinal = %d, want %d", i, item.Ordinal, wantOrdinal)
		}
	}
}

func TestCreateListItem_NotOwner_NotFound(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	owner := createTestUser(t, s, db, ctx, "lists-owner@example.com")
	other := createTestUser(t, s, db, ctx, "lists-other-teacher@example.com")

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{
		TeacherID: owner.ID,
		Title:     "Owned By Owner",
	})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, owner.ID) })

	_, err = s.CreateListItem(ctx, list.ID, other.ID, store.CreateListItemRequest{
		Title:      "Sneaky item",
		Difficulty: "easy",
		Body:       "Body",
	})
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound when a non-owner creates an item, got %v", err)
	}
}

func TestReorderListItems_AtomicOnPartialFailure(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "lists-reorder-teacher@example.com")

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{
		TeacherID: teacher.ID,
		Title:     "Reorder Test",
	})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })

	itemA, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{Title: "A", Difficulty: "easy", Body: "b"})
	if err != nil {
		t.Fatalf("CreateListItem A: %v", err)
	}
	itemB, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{Title: "B", Difficulty: "easy", Body: "b"})
	if err != nil {
		t.Fatalf("CreateListItem B: %v", err)
	}

	// The second entry targets a nonexistent item id — the whole batch must
	// roll back, leaving itemA's ordinal untouched.
	err = s.ReorderListItems(ctx, list.ID, teacher.ID, []store.ReorderItem{
		{ItemID: itemA.ID, Ordinal: 5},
		{ItemID: "00000000-0000-0000-0000-000000000000", Ordinal: 6},
	})
	if err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound from the batch, got %v", err)
	}

	detail, err := s.GetProblemListDetail(ctx, list.ID, teacher.ID, "teacher")
	if err != nil {
		t.Fatalf("GetProblemListDetail: %v", err)
	}
	for _, it := range detail.Items {
		if it.ID == itemA.ID && it.Ordinal != 0 {
			t.Errorf("itemA ordinal = %d, want 0 (unchanged — the batch should have rolled back)", it.Ordinal)
		}
	}
	_ = itemB
}

func TestListProblemListsForViewer_RoleVisibility(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "lists-visibility-teacher@example.com")
	student := createTestUser(t, s, db, ctx, "lists-visibility-student@example.com")

	draft, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Draft List"})
	if err != nil {
		t.Fatalf("CreateProblemList draft: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, draft.ID, teacher.ID) })

	published, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Published List"})
	if err != nil {
		t.Fatalf("CreateProblemList published: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, published.ID, teacher.ID) })
	if _, err := s.UpdateProblemList(ctx, published.ID, teacher.ID, store.UpdateProblemListRequest{
		Title: published.Title, Published: true,
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	teacherView, err := s.ListProblemListsForViewer(ctx, teacher.ID, "teacher")
	if err != nil {
		t.Fatalf("ListProblemListsForViewer (teacher): %v", err)
	}
	if !containsListID(teacherView, draft.ID) || !containsListID(teacherView, published.ID) {
		t.Error("teacher should see both their draft and published lists")
	}

	studentView, err := s.ListProblemListsForViewer(ctx, student.ID, "student")
	if err != nil {
		t.Fatalf("ListProblemListsForViewer (student): %v", err)
	}
	if containsListID(studentView, draft.ID) {
		t.Error("student must not see another teacher's draft list")
	}
	if !containsListID(studentView, published.ID) {
		t.Error("student should see the published list")
	}
}

func containsListID(lists []store.ProblemList, id string) bool {
	for _, l := range lists {
		if l.ID == id {
			return true
		}
	}
	return false
}

func TestGetProblemListDetail_DraftHiddenFromNonOwner(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "lists-draft-hidden-teacher@example.com")
	student := createTestUser(t, s, db, ctx, "lists-draft-hidden-student@example.com")

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Secret Draft"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })

	if _, err := s.GetProblemListDetail(ctx, list.ID, student.ID, "student"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for a student viewing a draft, got %v", err)
	}
	if _, err := s.GetProblemListDetail(ctx, list.ID, teacher.ID, "teacher"); err != nil {
		t.Errorf("owning teacher should be able to view their own draft, got %v", err)
	}
}

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("mustDate(%q): %v", s, err)
	}
	return d
}

// TestListProblemListsForViewer_OrderedByWeekStart covers the sort contract
// GET /lists relies on: newest week first, undated lists last ordered by
// created_at DESC among themselves.
func TestListProblemListsForViewer_OrderedByWeekStart(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "lists-order-teacher@example.com")

	older, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{
		TeacherID: teacher.ID, Title: "Older Week",
		WeekStart: new(mustDate(t, "2026-07-07")), WeekEnd: new(mustDate(t, "2026-07-13")),
	})
	if err != nil {
		t.Fatalf("CreateProblemList older: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, older.ID, teacher.ID) })

	newer, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{
		TeacherID: teacher.ID, Title: "Newer Week",
		WeekStart: new(mustDate(t, "2026-07-21")), WeekEnd: new(mustDate(t, "2026-07-27")),
	})
	if err != nil {
		t.Fatalf("CreateProblemList newer: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, newer.ID, teacher.ID) })

	undatedFirst, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{
		TeacherID: teacher.ID, Title: "Undated First",
	})
	if err != nil {
		t.Fatalf("CreateProblemList undatedFirst: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, undatedFirst.ID, teacher.ID) })

	undatedSecond, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{
		TeacherID: teacher.ID, Title: "Undated Second",
	})
	if err != nil {
		t.Fatalf("CreateProblemList undatedSecond: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, undatedSecond.ID, teacher.ID) })

	lists, err := s.ListProblemListsForViewer(ctx, teacher.ID, "teacher")
	if err != nil {
		t.Fatalf("ListProblemListsForViewer: %v", err)
	}

	// Filter down to just this test's four lists — the teacher may own
	// others left over from different tests sharing the same database.
	var ids []string
	for _, l := range lists {
		switch l.ID {
		case newer.ID, older.ID, undatedFirst.ID, undatedSecond.ID:
			ids = append(ids, l.ID)
		}
	}

	want := []string{newer.ID, older.ID, undatedSecond.ID, undatedFirst.ID}
	if len(ids) != len(want) {
		t.Fatalf("got %d relevant lists, want %d: %v", len(ids), len(want), ids)
	}
	for i, id := range ids {
		if id != want[i] {
			t.Errorf("position %d: got list %q, want %q", i, id, want[i])
		}
	}
}
