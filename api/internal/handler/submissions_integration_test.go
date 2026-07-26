//go:build integration

package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

func openHandlerTestDB(t *testing.T) *sql.DB {
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

// TestCreateSubmission_IsTestRunDerivedFromRole covers the actual business
// rule: is_test_run is never read from the request body (there's no such
// field on createSubmissionBody), it's derived server-side from the caller's
// JWT role — teacher submissions are test runs, student submissions are not.
func TestCreateSubmission_IsTestRunDerivedFromRole(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug:       "role-derives-test-run",
		Title:      "Role Derives Test Run",
		Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	// submissions.user_id is a UUID FK into users(id), so the JWT's sub claim
	// must be a real user row — the role claim is what the handler actually
	// derives is_test_run from, and that's independent of the user's stored
	// DB role, so one real user signed with two different role claims is
	// enough to isolate the decision under test.
	user, err := s.CreateUser(ctx, "role-derives-test-run@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, user.ID) })
	// Cleanups run LIFO: delete the submissions the tests below create before
	// either cleanup above runs, so their FKs (challenge_id, user_id) don't
	// block those deletes and orphan rows that collide on the next run.
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID) })

	j, err := auth.NewJWT([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("NewJWT: %v", err)
	}

	h := NewChallengesHandler(s)
	r := chi.NewRouter()
	r.With(j.Middleware).Post("/challenges/{id}/submissions", h.CreateSubmission)

	for _, tt := range []struct {
		name     string
		role     string
		wantTest bool
	}{
		{"teacher submission is a test run", "teacher", true},
		{"student submission is a real run", "student", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			token, err := j.Sign(user.ID, user.Email, tt.role, time.Now())
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}

			body := `{"language":"go","source_code":"package main\nfunc main() {}"}`
			req := httptest.NewRequest(http.MethodPost,
				"/challenges/"+ch.ID+"/submissions", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusAccepted {
				t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
			}

			var resp struct {
				SubmissionID string `json:"submission_id"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			var gotTestRun bool
			if err := db.QueryRowContext(ctx,
				`SELECT is_test_run FROM submissions WHERE id = $1`, resp.SubmissionID).Scan(&gotTestRun); err != nil {
				t.Fatalf("query is_test_run: %v", err)
			}
			if gotTestRun != tt.wantTest {
				t.Errorf("is_test_run = %v, want %v for role %q", gotTestRun, tt.wantTest, tt.role)
			}
		})
	}
}
