//go:build integration

package store_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/caze/ascend/api/internal/store"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateAndGetChallenge(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	req := store.CreateChallengeRequest{
		Slug:       "integration-test-slug",
		Title:      "Integration Test",
		Difficulty: "easy",
	}
	created, err := s.CreateChallenge(ctx, req)
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, created.ID) })

	if created.ID == "" {
		t.Error("expected non-empty ID")
	}
	if created.TimeLimitMs != 2000 {
		t.Errorf("expected default time_limit_ms=2000, got %d", created.TimeLimitMs)
	}
	if created.MemoryLimitMb != 256 {
		t.Errorf("expected default memory_limit_mb=256, got %d", created.MemoryLimitMb)
	}

	got, err := s.GetChallenge(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetChallenge: %v", err)
	}
	if got.Slug != req.Slug {
		t.Errorf("got slug %q, want %q", got.Slug, req.Slug)
	}
}

func TestGetChallenge_NotFound(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)

	_, err := s.GetChallenge(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListChallenges(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	req := store.CreateChallengeRequest{
		Slug:       "list-test-slug",
		Title:      "List Test",
		Difficulty: "hard",
	}
	created, err := s.CreateChallenge(ctx, req)
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, created.ID) })

	challenges, err := s.ListChallenges(ctx, 50, 0)
	if err != nil {
		t.Fatalf("ListChallenges: %v", err)
	}
	if len(challenges) == 0 {
		t.Error("expected at least one challenge")
	}
}

func TestDeleteChallenge_NotFound(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)

	err := s.DeleteChallenge(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateTestCase_OrdinalSequencing(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug:       "tc-ordinal-test",
		Title:      "TC Test",
		Difficulty: "medium",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	tc1, err := s.CreateTestCase(ctx, ch.ID, store.CreateTestCaseRequest{
		Input:          "1",
		ExpectedOutput: "1",
	})
	if err != nil {
		t.Fatalf("CreateTestCase 1: %v", err)
	}

	tc2, err := s.CreateTestCase(ctx, ch.ID, store.CreateTestCaseRequest{
		Input:          "2",
		ExpectedOutput: "2",
	})
	if err != nil {
		t.Fatalf("CreateTestCase 2: %v", err)
	}

	if tc2.Ordinal != tc1.Ordinal+1 {
		t.Errorf("expected ordinal %d, got %d", tc1.Ordinal+1, tc2.Ordinal)
	}

	tcs, err := s.ListTestCases(ctx, ch.ID)
	if err != nil {
		t.Fatalf("ListTestCases: %v", err)
	}
	if len(tcs) != 2 {
		t.Errorf("expected 2 test cases, got %d", len(tcs))
	}
	if tcs[0].Ordinal > tcs[1].Ordinal {
		t.Error("test cases not returned in ordinal order")
	}
}

func TestCreateTestCase_ChallengeNotFound(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)

	_, err := s.CreateTestCase(context.Background(), "00000000-0000-0000-0000-000000000000",
		store.CreateTestCaseRequest{ExpectedOutput: "3"})
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetChallengeDetail_SampleTestCases(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug:       "detail-sample-test",
		Title:      "Detail Test",
		Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	_, err = s.CreateTestCase(ctx, ch.ID, store.CreateTestCaseRequest{
		Input:          "1 2",
		ExpectedOutput: "3",
		IsSample:       true,
	})
	if err != nil {
		t.Fatalf("CreateTestCase (sample): %v", err)
	}

	_, err = s.CreateTestCase(ctx, ch.ID, store.CreateTestCaseRequest{
		Input:          "secret input",
		ExpectedOutput: "secret output",
		IsSample:       false,
	})
	if err != nil {
		t.Fatalf("CreateTestCase (private): %v", err)
	}

	detail, err := s.GetChallengeDetail(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetChallengeDetail: %v", err)
	}
	if detail.ID != ch.ID {
		t.Errorf("challenge id: got %q, want %q", detail.ID, ch.ID)
	}
	if len(detail.SampleTestCases) != 1 {
		t.Fatalf("sample test cases: got %d, want 1", len(detail.SampleTestCases))
	}
	if detail.SampleTestCases[0].Input != "1 2" {
		t.Errorf("sample input: got %q, want %q", detail.SampleTestCases[0].Input, "1 2")
	}
	if detail.SampleTestCases[0].ExpectedOutput != "3" {
		t.Errorf("sample expected_output: got %q, want %q", detail.SampleTestCases[0].ExpectedOutput, "3")
	}
}

func TestGetChallengeDetail_NotFound(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)

	_, err := s.GetChallengeDetail(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetChallengeDetail_EmptySamples(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug:       "detail-no-samples",
		Title:      "No Samples",
		Difficulty: "hard",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	detail, err := s.GetChallengeDetail(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetChallengeDetail: %v", err)
	}
	if detail.SampleTestCases == nil {
		t.Error("SampleTestCases must be an empty slice, not nil (JSON must serialize as [])")
	}
}
