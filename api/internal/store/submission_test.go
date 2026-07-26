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
