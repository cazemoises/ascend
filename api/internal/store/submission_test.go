//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/caze/ascend/api/internal/store"
)

func openTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("Redis unavailable (%v) — skipping integration test", err)
	}
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func TestCreateSubmission_HappyPath(t *testing.T) {
	db := openTestDB(t)
	rdb := openTestRedis(t)
	s := store.New(db, rdb)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug:       "sub-test-challenge",
		Title:      "Sub Test",
		Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	// Drain the submissions list before the test
	rdb.Del(ctx, "submissions")

	sub, err := s.CreateSubmission(ctx, store.CreateSubmissionRequest{
		ChallengeID: ch.ID,
		Language:    "go",
		SourceCode:  "package main\nfunc main() {}",
	})
	if err != nil {
		t.Fatalf("CreateSubmission: %v", err)
	}
	if sub.ID == "" {
		t.Error("expected non-empty submission ID")
	}
	if sub.Status != "pending" {
		t.Errorf("expected status pending, got %q", sub.Status)
	}
	if sub.ChallengeID != ch.ID {
		t.Errorf("challenge_id mismatch: got %q, want %q", sub.ChallengeID, ch.ID)
	}

	// Verify Redis enqueue
	msgs, err := rdb.LRange(ctx, "submissions", 0, -1).Result()
	if err != nil {
		t.Fatalf("LRANGE: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in Redis, got %d", len(msgs))
	}
	var payload struct {
		SubmissionID string `json:"submission_id"`
		ChallengeID  string `json:"challenge_id"`
	}
	if err := json.Unmarshal([]byte(msgs[0]), &payload); err != nil {
		t.Fatalf("unmarshal Redis payload: %v", err)
	}
	if payload.SubmissionID != sub.ID {
		t.Errorf("Redis submission_id %q != %q", payload.SubmissionID, sub.ID)
	}
	if payload.ChallengeID != ch.ID {
		t.Errorf("Redis challenge_id %q != %q", payload.ChallengeID, ch.ID)
	}
}

// TestCreateSubmission_QueueOrderIsFIFO guards the producer side of the
// submissions queue's ordering contract: the judge worker consumes with
// BLPop (pops from the list head), so CreateSubmission must RPush (append at
// the tail) for arrival order to be preserved. RPush+BLPop is FIFO; the bug
// this regression-tests was LPush+BLPop, which is LIFO — under a burst of
// submissions the worker would always grab the most recent one first,
// starving earlier submissions until the queue drained.
func TestCreateSubmission_QueueOrderIsFIFO(t *testing.T) {
	db := openTestDB(t)
	rdb := openTestRedis(t)
	s := store.New(db, rdb)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug:       "sub-test-fifo-challenge",
		Title:      "Sub Test FIFO",
		Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID) })

	rdb.Del(ctx, "submissions")

	var wantOrder []string
	for i := 0; i < 3; i++ {
		sub, err := s.CreateSubmission(ctx, store.CreateSubmissionRequest{
			ChallengeID: ch.ID,
			Language:    "go",
			SourceCode:  "package main\nfunc main() {}",
		})
		if err != nil {
			t.Fatalf("CreateSubmission %d: %v", i, err)
		}
		wantOrder = append(wantOrder, sub.ID)
	}

	// BLPop pops from the head (index 0), so simulate the worker draining
	// the queue by reading front-to-back with LRange.
	msgs, err := rdb.LRange(ctx, "submissions", 0, -1).Result()
	if err != nil {
		t.Fatalf("LRANGE: %v", err)
	}
	if len(msgs) != len(wantOrder) {
		t.Fatalf("expected %d messages in Redis, got %d", len(wantOrder), len(msgs))
	}
	for i, raw := range msgs {
		var payload struct {
			SubmissionID string `json:"submission_id"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			t.Fatalf("unmarshal Redis payload %d: %v", i, err)
		}
		if payload.SubmissionID != wantOrder[i] {
			t.Errorf("queue position %d: got submission %q, want %q (arrival order not preserved)",
				i, payload.SubmissionID, wantOrder[i])
		}
	}
}

// TestCreateSubmission_IsTestRunPersisted covers the store's half of the
// teacher-test-run flag: whatever the caller passes in IsTestRun must land
// unchanged in the is_test_run column. Deriving true/false from the JWT role
// is the handler's job (covered in the handler package); the store just has
// to persist the bit faithfully.
func TestCreateSubmission_IsTestRunPersisted(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug:       "is-test-run-persisted",
		Title:      "Is Test Run Persisted",
		Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })
	// Cleanups run LIFO: delete the submissions created below before the
	// DeleteChallenge above runs, so the FK doesn't block it and orphan the
	// challenge row (which would then collide on its unique slug next run).
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID) })

	for _, tt := range []struct {
		name      string
		isTestRun bool
	}{
		{"teacher test run", true},
		{"student real run", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sub, err := s.CreateSubmission(ctx, store.CreateSubmissionRequest{
				ChallengeID: ch.ID,
				Language:    "go",
				SourceCode:  "package main\nfunc main() {}",
				IsTestRun:   tt.isTestRun,
			})
			if err != nil {
				t.Fatalf("CreateSubmission: %v", err)
			}

			var got bool
			if err := db.QueryRowContext(ctx,
				`SELECT is_test_run FROM submissions WHERE id = $1`, sub.ID).Scan(&got); err != nil {
				t.Fatalf("query is_test_run: %v", err)
			}
			if got != tt.isTestRun {
				t.Errorf("is_test_run = %v, want %v", got, tt.isTestRun)
			}
		})
	}
}

// TestGetSubmission_ExposesStdoutAndExpectedOutput covers the store's read
// side of the worker's wrong-answer snapshot: once stdout/expected_output
// are written directly (simulating what the judge worker does), GetSubmission
// must scan them back onto the Submission struct.
func TestGetSubmission_ExposesStdoutAndExpectedOutput(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "get-submission-output", Title: "Get Submission Output", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID) })

	sub, err := s.CreateSubmission(ctx, store.CreateSubmissionRequest{
		ChallengeID: ch.ID, Language: "python", SourceCode: "print('nope')",
	})
	if err != nil {
		t.Fatalf("CreateSubmission: %v", err)
	}

	// Before judging, both columns are NULL.
	before, err := s.GetSubmission(ctx, sub.ID)
	if err != nil {
		t.Fatalf("GetSubmission (before): %v", err)
	}
	if before.Stdout != nil || before.ExpectedOutput != nil {
		t.Errorf("stdout=%v expected_output=%v, want both nil before judging", before.Stdout, before.ExpectedOutput)
	}

	if _, err := db.ExecContext(ctx,
		`UPDATE submissions SET status = 'wrong_answer', stdout = $1, expected_output = $2 WHERE id = $3`,
		"nope", "expected value", sub.ID); err != nil {
		t.Fatalf("simulate worker update: %v", err)
	}

	after, err := s.GetSubmission(ctx, sub.ID)
	if err != nil {
		t.Fatalf("GetSubmission (after): %v", err)
	}
	if after.Stdout == nil || *after.Stdout != "nope" {
		t.Errorf("stdout = %v, want %q", after.Stdout, "nope")
	}
	if after.ExpectedOutput == nil || *after.ExpectedOutput != "expected value" {
		t.Errorf("expected_output = %v, want %q", after.ExpectedOutput, "expected value")
	}
}

// TestGetSubmission_ExposesFailedInputAndIsSample covers the same
// worker-writes/store-reads contract as the stdout/expected_output test
// above, for the two new columns — including the specific case the whole
// feature exists for: a hidden test case's input must come back nil even
// though failed_is_sample itself is populated (false, not NULL).
func TestGetSubmission_ExposesFailedInputAndIsSample(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "get-submission-failed-input", Title: "Get Submission Failed Input", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID) })

	sub, err := s.CreateSubmission(ctx, store.CreateSubmissionRequest{
		ChallengeID: ch.ID, Language: "python", SourceCode: "print('nope')",
	})
	if err != nil {
		t.Fatalf("CreateSubmission: %v", err)
	}

	// Before judging (and for any submission predating this column), both
	// must be nil — never a misleading false.
	before, err := s.GetSubmission(ctx, sub.ID)
	if err != nil {
		t.Fatalf("GetSubmission (before): %v", err)
	}
	if before.FailedInput != nil || before.FailedIsSample != nil {
		t.Errorf("failed_input=%v failed_is_sample=%v, want both nil before judging", before.FailedInput, before.FailedIsSample)
	}

	// Simulate the worker judging a wrong_answer on a SAMPLE case.
	if _, err := db.ExecContext(ctx,
		`UPDATE submissions SET status = 'wrong_answer', failed_input = $1, failed_is_sample = $2 WHERE id = $3`,
		"3 4", true, sub.ID); err != nil {
		t.Fatalf("simulate worker update (sample case): %v", err)
	}
	afterSample, err := s.GetSubmission(ctx, sub.ID)
	if err != nil {
		t.Fatalf("GetSubmission (after sample): %v", err)
	}
	if afterSample.FailedIsSample == nil || !*afterSample.FailedIsSample {
		t.Errorf("failed_is_sample = %v, want true", afterSample.FailedIsSample)
	}
	if afterSample.FailedInput == nil || *afterSample.FailedInput != "3 4" {
		t.Errorf("failed_input = %v, want %q", afterSample.FailedInput, "3 4")
	}

	// Simulate the worker judging a wrong_answer on a HIDDEN case: input
	// must come back nil even though failed_is_sample is populated (false).
	if _, err := db.ExecContext(ctx,
		`UPDATE submissions SET status = 'wrong_answer', failed_input = NULL, failed_is_sample = $1 WHERE id = $2`,
		false, sub.ID); err != nil {
		t.Fatalf("simulate worker update (hidden case): %v", err)
	}
	afterHidden, err := s.GetSubmission(ctx, sub.ID)
	if err != nil {
		t.Fatalf("GetSubmission (after hidden): %v", err)
	}
	if afterHidden.FailedIsSample == nil || *afterHidden.FailedIsSample {
		t.Errorf("failed_is_sample = %v, want false", afterHidden.FailedIsSample)
	}
	if afterHidden.FailedInput != nil {
		t.Errorf("failed_input = %v, want nil — a hidden case's input must never be exposed", afterHidden.FailedInput)
	}
}

func TestCreateSubmission_ChallengeNotFound(t *testing.T) {
	db := openTestDB(t)
	rdb := openTestRedis(t)
	s := store.New(db, rdb)

	_, err := s.CreateSubmission(context.Background(), store.CreateSubmissionRequest{
		ChallengeID: "00000000-0000-0000-0000-000000000000",
		Language:    "python",
		SourceCode:  "print('hi')",
	})
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
