//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/caze/ascend/api/internal/store"
)

func findStudentOverview(overview []store.StudentOverview, studentID string) (store.StudentOverview, bool) {
	for _, so := range overview {
		if so.StudentID == studentID {
			return so, true
		}
	}
	return store.StudentOverview{}, false
}

func TestTeacherStudentsOverview_BestVerdictAndNoInteraction(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	teacher := createTestUser(t, s, db, ctx, "overview-teacher@example.com")
	solver := createTestUser(t, s, db, ctx, "overview-solver@example.com")
	strugglingStudent := createTestUser(t, s, db, ctx, "overview-struggling@example.com")
	uninvolvedStudent := createTestUser(t, s, db, ctx, "overview-uninvolved@example.com")

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "overview-test-challenge", Title: "Overview Test Challenge", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID)
		s.DeleteChallenge(ctx, ch.ID)
	})

	// solver: wrong_answer then accepted (older first) -> best verdict must
	// be "accepted", not the most recent status, since an accepted run exists.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO submissions (challenge_id, user_id, language, source_code, status, is_test_run, created_at)
		 VALUES ($1, $2, 'go', 'x', 'wrong_answer', false, now() - interval '1 hour')`, ch.ID, solver.ID); err != nil {
		t.Fatalf("insert solver first submission: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO submissions (challenge_id, user_id, language, source_code, status, is_test_run, created_at)
		 VALUES ($1, $2, 'go', 'x', 'accepted', false, now())`, ch.ID, solver.ID); err != nil {
		t.Fatalf("insert solver second submission: %v", err)
	}

	// strugglingStudent: only ever wrong_answer -> best verdict is the most
	// recent (only) status, "wrong_answer".
	if _, err := db.ExecContext(ctx,
		`INSERT INTO submissions (challenge_id, user_id, language, source_code, status, is_test_run)
		 VALUES ($1, $2, 'go', 'x', 'wrong_answer', false)`, ch.ID, strugglingStudent.ID); err != nil {
		t.Fatalf("insert strugglingStudent submission: %v", err)
	}

	// teacher's own test-run submission must never surface anyone in the
	// overview — it isn't a student, and is_test_run=true is excluded anyway.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO submissions (challenge_id, user_id, language, source_code, status, is_test_run)
		 VALUES ($1, $2, 'go', 'x', 'accepted', true)`, ch.ID, teacher.ID); err != nil {
		t.Fatalf("insert teacher test-run submission: %v", err)
	}

	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID)
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, solver.ID)
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, strugglingStudent.ID)
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, uninvolvedStudent.ID)
	})

	overview, err := s.TeacherStudentsOverview(ctx)
	if err != nil {
		t.Fatalf("TeacherStudentsOverview: %v", err)
	}

	solverRow, ok := findStudentOverview(overview, solver.ID)
	if !ok {
		t.Fatal("solver missing from overview")
	}
	if len(solverRow.Challenges) != 1 || solverRow.Challenges[0].BestVerdict != "accepted" {
		t.Errorf("solver challenges = %+v, want single entry with best_verdict=accepted", solverRow.Challenges)
	}

	strugglingRow, ok := findStudentOverview(overview, strugglingStudent.ID)
	if !ok {
		t.Fatal("strugglingStudent missing from overview")
	}
	if len(strugglingRow.Challenges) != 1 || strugglingRow.Challenges[0].BestVerdict != "wrong_answer" {
		t.Errorf("strugglingStudent challenges = %+v, want single entry with best_verdict=wrong_answer", strugglingRow.Challenges)
	}

	if _, ok := findStudentOverview(overview, uninvolvedStudent.ID); ok {
		t.Error("uninvolvedStudent (no submissions, no completions) must not appear in the overview")
	}

	if _, ok := findStudentOverview(overview, teacher.ID); ok {
		t.Error("teacher's own test-run submission must not surface the teacher as a student row")
	}
}

func TestTeacherStudentsOverview_IncludesListCompletionsOnly(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	teacher := createTestUser(t, s, db, ctx, "overview-list-teacher@example.com")
	student := createTestUser(t, s, db, ctx, "overview-list-student@example.com")

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Overview List"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })
	if _, err := s.UpdateProblemList(ctx, list.ID, teacher.ID, store.UpdateProblemListRequest{Title: list.Title, Published: true}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{Title: "Overview Item", Difficulty: "easy", Body: "b"})
	if err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}
	if err := s.CompleteListItem(ctx, item.ID, student.ID); err != nil {
		t.Fatalf("CompleteListItem: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID)
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, student.ID)
	})

	overview, err := s.TeacherStudentsOverview(ctx)
	if err != nil {
		t.Fatalf("TeacherStudentsOverview: %v", err)
	}

	row, ok := findStudentOverview(overview, student.ID)
	if !ok {
		t.Fatal("student with only a list completion (no submissions) must still appear in the overview")
	}
	if len(row.Challenges) != 0 {
		t.Errorf("challenges = %+v, want none", row.Challenges)
	}
	if len(row.Completions) != 1 || row.Completions[0].ListTitle != "Overview List" || row.Completions[0].ItemTitle != "Overview Item" {
		t.Errorf("completions = %+v, want one entry for Overview List / Overview Item", row.Completions)
	}
}
