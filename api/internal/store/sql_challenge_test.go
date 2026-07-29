//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/caze/ascend/api/internal/store"
)

func TestCreateChallenge_PersistsLanguageAndSQLSchema(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	language := "sql"
	schema := "CREATE TABLE students(id INT, name TEXT);"
	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "sql-challenge-test", Title: "SQL Challenge", Difficulty: "easy",
		Language: &language, SQLSchema: &schema,
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	if ch.Language == nil || *ch.Language != "sql" {
		t.Errorf("Language = %v, want \"sql\"", ch.Language)
	}
	if ch.SQLSchema == nil || *ch.SQLSchema != schema {
		t.Errorf("SQLSchema = %v, want %q", ch.SQLSchema, schema)
	}

	got, err := s.GetChallenge(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetChallenge: %v", err)
	}
	if got.Language == nil || *got.Language != "sql" {
		t.Errorf("GetChallenge Language = %v, want \"sql\"", got.Language)
	}
}

func TestCreateChallenge_LanguageDefaultsNil(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "multi-lang-challenge-test", Title: "Multi Lang", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	if ch.Language != nil {
		t.Errorf("Language = %v, want nil (today's multi-language behavior)", *ch.Language)
	}
}

func TestCreateTestCase_PersistsOrderMatters(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "order-matters-test", Title: "Order Matters", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	tc, err := s.CreateTestCase(ctx, ch.ID, store.CreateTestCaseRequest{
		ExpectedOutput: "1|x", OrderMatters: true,
	})
	if err != nil {
		t.Fatalf("CreateTestCase: %v", err)
	}
	if !tc.OrderMatters {
		t.Error("OrderMatters = false, want true")
	}

	tcs, err := s.ListTestCases(ctx, ch.ID)
	if err != nil {
		t.Fatalf("ListTestCases: %v", err)
	}
	if len(tcs) != 1 || !tcs[0].OrderMatters {
		t.Errorf("ListTestCases OrderMatters not persisted: %+v", tcs)
	}
}

func TestCreateSubmission_RejectsSQLOnMultiLanguageChallenge(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "lang-mismatch-multi-test", Title: "Multi Lang", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	_, err = s.CreateSubmission(ctx, store.CreateSubmissionRequest{
		ChallengeID: ch.ID, Language: "sql", SourceCode: "SELECT 1;",
	})
	if err != store.ErrLanguageMismatch {
		t.Errorf("err = %v, want ErrLanguageMismatch", err)
	}
}

func TestCreateSubmission_RejectsNonSQLOnSQLOnlyChallenge(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	language := "sql"
	schema := "CREATE TABLE t(a INT);"
	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "lang-mismatch-sql-test", Title: "SQL Only", Difficulty: "easy",
		Language: &language, SQLSchema: &schema,
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	_, err = s.CreateSubmission(ctx, store.CreateSubmissionRequest{
		ChallengeID: ch.ID, Language: "python", SourceCode: "print(1)",
	})
	if err != store.ErrLanguageMismatch {
		t.Errorf("err = %v, want ErrLanguageMismatch", err)
	}
}

func TestCreateSubmission_AcceptsSQLOnSQLOnlyChallenge(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	language := "sql"
	schema := "CREATE TABLE t(a INT);"
	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "lang-match-sql-test", Title: "SQL Only", Difficulty: "easy",
		Language: &language, SQLSchema: &schema,
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID)
		s.DeleteChallenge(ctx, ch.ID)
	})

	sub, err := s.CreateSubmission(ctx, store.CreateSubmissionRequest{
		ChallengeID: ch.ID, Language: "sql", SourceCode: "SELECT * FROM t;",
	})
	if err != nil {
		t.Fatalf("CreateSubmission: %v", err)
	}
	if sub.Language != "sql" {
		t.Errorf("Language = %q, want sql", sub.Language)
	}
}
