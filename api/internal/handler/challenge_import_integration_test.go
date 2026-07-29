//go:build integration

package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/lib/pq"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

func newImportRouter(s *store.Store) chi.Router {
	h := NewChallengesHandler(s)
	r := chi.NewRouter()
	r.With(auth.RequireAuthenticated, auth.RequireRole("teacher")).
		Post("/challenges/import", h.Import)
	return r
}

func doImport(t *testing.T, r chi.Router, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/challenges/import", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.NewContext(req.Context(),
		auth.Claims{UserID: "teacher-1", Email: "teacher@example.com", Role: "teacher"}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// cleanupImportedSlugs deletes any challenges (and their test cases) with
// the given slugs, regardless of whether the import under test actually
// persisted them — safe to call unconditionally in t.Cleanup.
func cleanupImportedSlugs(t *testing.T, db *sql.DB, ctx context.Context, slugs ...string) {
	t.Helper()
	rows, err := db.QueryContext(ctx, `SELECT id FROM challenges WHERE slug = ANY($1)`, pq.Array(slugs))
	if err != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, id)
		db.ExecContext(ctx, `DELETE FROM test_cases WHERE challenge_id = $1`, id)
		db.ExecContext(ctx, `DELETE FROM challenges WHERE id = $1`, id)
	}
}

func TestImportChallenges_SingleValid(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	r := newImportRouter(s)

	t.Cleanup(func() { cleanupImportedSlugs(t, db, ctx, "import-single-valid") })

	body := `{
		"challenges": [
			{
				"slug": "import-single-valid",
				"title": "Import Single Valid",
				"description": "desc",
				"difficulty": "easy",
				"test_cases": [
					{"input": "1 2", "expected_output": "3", "is_sample": true}
				]
			}
		]
	}`

	w := doImport(t, r, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body.String())
	}

	var results []struct {
		Slug      string `json:"slug"`
		TestCases []struct {
			ExpectedOutput string `json:"expected_output"`
			Ordinal        int    `json:"ordinal"`
		} `json:"test_cases"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 challenge in response, got %d", len(results))
	}
	if results[0].Slug != "import-single-valid" {
		t.Errorf("slug = %q, want import-single-valid", results[0].Slug)
	}
	if len(results[0].TestCases) != 1 || results[0].TestCases[0].ExpectedOutput != "3" {
		t.Errorf("test_cases = %+v, want one case with expected_output 3", results[0].TestCases)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM challenges WHERE slug = $1`, "import-single-valid").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("persisted count = %d, want 1", count)
	}
}

func TestImportChallenges_InvalidItemRollsBackEverything(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	r := newImportRouter(s)

	slugs := []string{"import-rollback-1", "import-rollback-2", "import-rollback-3"}
	t.Cleanup(func() { cleanupImportedSlugs(t, db, ctx, slugs...) })

	body := `{
		"challenges": [
			{"slug": "import-rollback-1", "title": "One", "description": "d", "difficulty": "easy"},
			{"slug": "import-rollback-2", "title": "Two", "description": "d", "difficulty": "impossible"},
			{"slug": "import-rollback-3", "title": "Three", "description": "d", "difficulty": "hard"}
		]
	}`

	w := doImport(t, r, body)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); !bytes.Contains([]byte(got), []byte("challenge 1")) {
		t.Errorf("error message %q does not identify challenge 1 (0-indexed)", got)
	}

	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM challenges WHERE slug = ANY($1)`, pq.Array(slugs)).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("persisted count = %d, want 0 (full rollback)", count)
	}
}

func TestImportChallenges_SQLWithoutSchemaRejected(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	r := newImportRouter(s)

	t.Cleanup(func() { cleanupImportedSlugs(t, db, ctx, "import-sql-no-schema") })

	body := `{
		"challenges": [
			{
				"slug": "import-sql-no-schema",
				"title": "SQL No Schema",
				"description": "d",
				"difficulty": "easy",
				"language": "sql"
			}
		]
	}`

	w := doImport(t, r, body)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	// Same message validateSQLFields produces for the regular Create endpoint —
	// proves the import path reuses it rather than a divergent copy.
	want := `sql_schema is required when language is "sql"`
	if !strings.Contains(resp.Error, want) {
		t.Errorf("error = %q, want it to contain %q", resp.Error, want)
	}
}

func TestImportChallenges_DuplicateSlugWithinBatchRejected(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	r := newImportRouter(s)

	t.Cleanup(func() { cleanupImportedSlugs(t, db, ctx, "import-dup-batch") })

	body := `{
		"challenges": [
			{"slug": "import-dup-batch", "title": "First", "description": "d", "difficulty": "easy"},
			{"slug": "import-dup-batch", "title": "Second", "description": "d", "difficulty": "easy"}
		]
	}`

	w := doImport(t, r, body)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", w.Code, w.Body.String())
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM challenges WHERE slug = $1`, "import-dup-batch").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("persisted count = %d, want 0 (full rollback, including the first item)", count)
	}
}

func TestImportChallenges_DuplicateSlugAgainstExistingRejected(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	r := newImportRouter(s)

	existing, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "import-dup-existing", Title: "Existing", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, existing.ID) })

	body := `{
		"challenges": [
			{"slug": "import-dup-existing", "title": "New", "description": "d", "difficulty": "easy"}
		]
	}`

	w := doImport(t, r, body)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", w.Code, w.Body.String())
	}
}
