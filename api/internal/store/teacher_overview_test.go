//go:build integration

package store_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

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

func findChallengeSummary(challenges []store.StudentChallengeSummary, challengeID string) (store.StudentChallengeSummary, bool) {
	for _, cs := range challenges {
		if cs.ChallengeID == challengeID {
			return cs, true
		}
	}
	return store.StudentChallengeSummary{}, false
}

func insertSubmission(t *testing.T, db *sql.DB, ctx context.Context, challengeID, userID, status string, createdAt time.Time) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO submissions (challenge_id, user_id, language, source_code, status, is_test_run, created_at)
		 VALUES ($1, $2, 'go', 'x', $3, false, $4)`,
		challengeID, userID, status, createdAt); err != nil {
		t.Fatalf("insert submission (status=%s): %v", status, err)
	}
}

func TestTeacherStudentsOverview_ChallengeStatsAndChronologicalOrder(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	student := createTestUser(t, s, db, ctx, "overview-attempts-student@example.com")

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "overview-attempts-challenge", Title: "Overview Attempts Challenge", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID)
		s.DeleteChallenge(ctx, ch.ID)
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, student.ID)
	})

	base := time.Now().Add(-time.Hour)
	insertSubmission(t, db, ctx, ch.ID, student.ID, "wrong_answer", base)
	insertSubmission(t, db, ctx, ch.ID, student.ID, "runtime_error", base.Add(1*time.Minute))
	insertSubmission(t, db, ctx, ch.ID, student.ID, "accepted", base.Add(2*time.Minute))

	overview, err := s.TeacherStudentsOverview(ctx)
	if err != nil {
		t.Fatalf("TeacherStudentsOverview: %v", err)
	}

	row, ok := findStudentOverview(overview, student.ID)
	if !ok {
		t.Fatal("student missing from overview")
	}
	cs, ok := findChallengeSummary(row.Challenges, ch.ID)
	if !ok {
		t.Fatal("challenge missing from student's challenges")
	}

	if cs.TotalAttempts != 3 {
		t.Errorf("TotalAttempts = %d, want 3", cs.TotalAttempts)
	}
	if cs.AcceptedCount != 1 || cs.WrongAnswerCount != 1 || cs.RuntimeErrorCount != 1 || cs.TimeoutCount != 0 {
		t.Errorf("verdict counts = %+v, want accepted=1 wrong_answer=1 runtime_error=1 timeout=0", cs)
	}
	if len(cs.Attempts) != 3 {
		t.Fatalf("len(Attempts) = %d, want 3", len(cs.Attempts))
	}
	wantOrder := []string{"wrong_answer", "runtime_error", "accepted"}
	for i, want := range wantOrder {
		if cs.Attempts[i].Verdict != want {
			t.Errorf("Attempts[%d].Verdict = %s, want %s (attempts must be chronological)", i, cs.Attempts[i].Verdict, want)
		}
	}
}

func TestTeacherStudentsOverview_AvgAttemptsToSolveIgnoresUnsolvedChallenges(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	student := createTestUser(t, s, db, ctx, "overview-avg-student@example.com")

	solved, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "overview-avg-solved", Title: "Overview Avg Solved", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge solved: %v", err)
	}
	unsolved, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "overview-avg-unsolved", Title: "Overview Avg Unsolved", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge unsolved: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1 OR challenge_id = $2`, solved.ID, unsolved.ID)
		s.DeleteChallenge(ctx, solved.ID)
		s.DeleteChallenge(ctx, unsolved.ID)
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, student.ID)
	})

	base := time.Now().Add(-time.Hour)
	// Solved on the 2nd attempt.
	insertSubmission(t, db, ctx, solved.ID, student.ID, "wrong_answer", base)
	insertSubmission(t, db, ctx, solved.ID, student.ID, "accepted", base.Add(time.Minute))
	// Never solved — must not pull the average down or count as accepted.
	insertSubmission(t, db, ctx, unsolved.ID, student.ID, "wrong_answer", base)
	insertSubmission(t, db, ctx, unsolved.ID, student.ID, "wrong_answer", base.Add(time.Minute))
	insertSubmission(t, db, ctx, unsolved.ID, student.ID, "wrong_answer", base.Add(2*time.Minute))

	overview, err := s.TeacherStudentsOverview(ctx)
	if err != nil {
		t.Fatalf("TeacherStudentsOverview: %v", err)
	}

	row, ok := findStudentOverview(overview, student.ID)
	if !ok {
		t.Fatal("student missing from overview")
	}

	if row.Stats.ChallengesAttempted != 2 {
		t.Errorf("ChallengesAttempted = %d, want 2", row.Stats.ChallengesAttempted)
	}
	if row.Stats.ChallengesAccepted != 1 {
		t.Errorf("ChallengesAccepted = %d, want 1", row.Stats.ChallengesAccepted)
	}
	if row.Stats.TotalSubmissions != 5 {
		t.Errorf("TotalSubmissions = %d, want 5", row.Stats.TotalSubmissions)
	}
	switch {
	case row.Stats.AvgAttemptsToSolve == nil:
		t.Error("AvgAttemptsToSolve = nil, want 2 (only the solved challenge counts, the unsolved one is excluded)")
	case *row.Stats.AvgAttemptsToSolve != 2.0:
		t.Errorf("AvgAttemptsToSolve = %v, want 2 (only the solved challenge counts, the unsolved one is excluded)", *row.Stats.AvgAttemptsToSolve)
	}
}

func TestTeacherStudentsOverview_AvgAttemptsToSolveNilWhenNothingSolved(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	student := createTestUser(t, s, db, ctx, "overview-avg-nilstudent@example.com")
	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "overview-avg-nil-challenge", Title: "Overview Avg Nil Challenge", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID)
		s.DeleteChallenge(ctx, ch.ID)
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, student.ID)
	})
	insertSubmission(t, db, ctx, ch.ID, student.ID, "wrong_answer", time.Now())

	overview, err := s.TeacherStudentsOverview(ctx)
	if err != nil {
		t.Fatalf("TeacherStudentsOverview: %v", err)
	}
	row, ok := findStudentOverview(overview, student.ID)
	if !ok {
		t.Fatal("student missing from overview")
	}
	if row.Stats.AvgAttemptsToSolve != nil {
		t.Errorf("AvgAttemptsToSolve = %v, want nil (no challenge solved)", *row.Stats.AvgAttemptsToSolve)
	}
}

func TestTeacherStudentsOverview_UninvolvedStudentAndTeacherTestRunExcluded(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	teacher := createTestUser(t, s, db, ctx, "overview-teacher@example.com")
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
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID)
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, uninvolvedStudent.ID)
	})

	// The teacher's own test-run submission must never surface anyone in the
	// overview — it isn't a student, and is_test_run=true is excluded anyway.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO submissions (challenge_id, user_id, language, source_code, status, is_test_run)
		 VALUES ($1, $2, 'go', 'x', 'accepted', true)`, ch.ID, teacher.ID); err != nil {
		t.Fatalf("insert teacher test-run submission: %v", err)
	}

	overview, err := s.TeacherStudentsOverview(ctx)
	if err != nil {
		t.Fatalf("TeacherStudentsOverview: %v", err)
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
	if row.Stats.TotalSubmissions != 0 || row.Stats.ChallengesAttempted != 0 || row.Stats.AvgAttemptsToSolve != nil {
		t.Errorf("stats = %+v, want all zero/nil for a student with no submissions", row.Stats)
	}
	if len(row.Completions) != 1 || row.Completions[0].ListTitle != "Overview List" || row.Completions[0].ItemTitle != "Overview Item" {
		t.Errorf("completions = %+v, want one entry for Overview List / Overview Item", row.Completions)
	}
}
