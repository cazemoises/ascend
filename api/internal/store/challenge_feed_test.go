//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

func findFeedItem(t *testing.T, items []store.ChallengeFeedItem, challengeID string) store.ChallengeFeedItem {
	t.Helper()
	for _, item := range items {
		if item.ID == challengeID {
			return item
		}
	}
	t.Fatalf("challenge %s not found in feed", challengeID)
	return store.ChallengeFeedItem{}
}

func TestListChallengesForViewer_Solved(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "solved-flag-test", Title: "Solved Flag Test", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}

	hash, err := auth.HashPassword("unused")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	solver, err := s.CreateUser(ctx, "solved-flag-solver@example.com", hash)
	if err != nil {
		t.Fatalf("CreateUser solver: %v", err)
	}

	teacher, err := s.CreateUser(ctx, "solved-flag-teacher@example.com", hash)
	if err != nil {
		t.Fatalf("CreateUser teacher: %v", err)
	}

	nonSolver, err := s.CreateUser(ctx, "solved-flag-nonsolver@example.com", hash)
	if err != nil {
		t.Fatalf("CreateUser nonSolver: %v", err)
	}

	// Submissions carry FKs to both the challenge and the users above, so
	// they must be deleted before either — a single cleanup in this fixed
	// order, rather than relying on t.Cleanup's LIFO registration order.
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID)
		s.DeleteChallenge(ctx, ch.ID)
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, solver.ID)
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID)
		db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, nonSolver.ID)
	})

	// solver: accepted, real run -> solved
	if _, err := db.ExecContext(ctx,
		`INSERT INTO submissions (challenge_id, user_id, language, source_code, status, is_test_run)
		 VALUES ($1, $2, 'go', 'x', 'accepted', false)`, ch.ID, solver.ID); err != nil {
		t.Fatalf("insert solver submission: %v", err)
	}
	// teacher: accepted, but a test run -> must not count as solved
	if _, err := db.ExecContext(ctx,
		`INSERT INTO submissions (challenge_id, user_id, language, source_code, status, is_test_run)
		 VALUES ($1, $2, 'go', 'x', 'accepted', true)`, ch.ID, teacher.ID); err != nil {
		t.Fatalf("insert teacher submission: %v", err)
	}
	// nonSolver: wrong_answer, real run -> not solved
	if _, err := db.ExecContext(ctx,
		`INSERT INTO submissions (challenge_id, user_id, language, source_code, status, is_test_run)
		 VALUES ($1, $2, 'go', 'x', 'wrong_answer', false)`, ch.ID, nonSolver.ID); err != nil {
		t.Fatalf("insert nonSolver submission: %v", err)
	}

	t.Run("solver sees solved=true", func(t *testing.T) {
		items, err := s.ListChallengesForViewer(ctx, solver.ID, 100, 0)
		if err != nil {
			t.Fatalf("ListChallengesForViewer: %v", err)
		}
		if got := findFeedItem(t, items, ch.ID); !got.Solved {
			t.Errorf("solved = %v, want true", got.Solved)
		}
	})

	t.Run("teacher's own accepted test run does not count as solved", func(t *testing.T) {
		items, err := s.ListChallengesForViewer(ctx, teacher.ID, 100, 0)
		if err != nil {
			t.Fatalf("ListChallengesForViewer: %v", err)
		}
		if got := findFeedItem(t, items, ch.ID); got.Solved {
			t.Errorf("solved = %v, want false (is_test_run=true doesn't count)", got.Solved)
		}
	})

	t.Run("non-solver sees solved=false", func(t *testing.T) {
		items, err := s.ListChallengesForViewer(ctx, nonSolver.ID, 100, 0)
		if err != nil {
			t.Fatalf("ListChallengesForViewer: %v", err)
		}
		if got := findFeedItem(t, items, ch.ID); got.Solved {
			t.Errorf("solved = %v, want false (no accepted submission)", got.Solved)
		}
	})

	t.Run("anonymous viewer sees solved=false", func(t *testing.T) {
		items, err := s.ListChallengesForViewer(ctx, "", 100, 0)
		if err != nil {
			t.Fatalf("ListChallengesForViewer: %v", err)
		}
		if got := findFeedItem(t, items, ch.ID); got.Solved {
			t.Errorf("solved = %v, want false (no viewer identity)", got.Solved)
		}
	})
}
