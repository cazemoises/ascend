//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/caze/ascend/api/internal/store"
)

func TestGetChallengeStats_ExcludesTestRuns(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug:       "stats-excludes-test-runs",
		Title:      "Stats Excludes Test Runs",
		Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })
	// Cleanups run LIFO: this deletes the submissions before the
	// DeleteChallenge above runs, so the FK doesn't block it and orphan the
	// challenge row (which would then collide on its unique slug next run).
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID) })

	// A teacher's test run: fast (10ms) so it would badly skew the
	// percentile/bucket math if counted, and marked accepted directly
	// (bypassing the judge, which isn't running in this test).
	teacherSub, err := s.CreateSubmission(ctx, store.CreateSubmissionRequest{
		ChallengeID: ch.ID,
		Language:    "go",
		SourceCode:  "package main\nfunc main() {}",
		IsTestRun:   true,
	})
	if err != nil {
		t.Fatalf("CreateSubmission (teacher): %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE submissions SET status = 'accepted', exec_time_ms = $1 WHERE id = $2`,
		10, teacherSub.ID); err != nil {
		t.Fatalf("mark teacher submission accepted: %v", err)
	}

	// A real student submission: slower (500ms), the only one that should
	// count toward TotalRuns and the percentile.
	studentSub, err := s.CreateSubmission(ctx, store.CreateSubmissionRequest{
		ChallengeID: ch.ID,
		Language:    "go",
		SourceCode:  "package main\nfunc main() {}",
		IsTestRun:   false,
	})
	if err != nil {
		t.Fatalf("CreateSubmission (student): %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE submissions SET status = 'accepted', exec_time_ms = $1 WHERE id = $2`,
		500, studentSub.ID); err != nil {
		t.Fatalf("mark student submission accepted: %v", err)
	}

	// An arbitrary well-formed UUID: GetChallengeStats' second query looks up
	// this specific user's best time, which is irrelevant to what this test
	// checks (TotalRuns/bucket exclusion), but the column is a non-nullable
	// uuid FK-typed parameter, so it must at least parse.
	stats, err := s.GetChallengeStats(ctx, ch.ID, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("GetChallengeStats: %v", err)
	}

	if stats.TotalRuns != 1 {
		t.Errorf("TotalRuns = %d, want 1 (teacher test run must be excluded)", stats.TotalRuns)
	}

	maxBucketMs := 0
	for _, b := range stats.Buckets {
		if b.Count > 0 && b.UpToMs > maxBucketMs {
			maxBucketMs = b.UpToMs
		}
	}
	if maxBucketMs > 0 && maxBucketMs < 400 {
		t.Errorf("bucket histogram looks skewed toward the excluded 10ms test run (max populated bucket up_to_ms=%d)", maxBucketMs)
	}
}
