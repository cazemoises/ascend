//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

func TestGetLastSubmission(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "last-submission-handler", Title: "Last Submission Handler", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	student, err := s.CreateUser(ctx, "last-submission-student@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser (student): %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, student.ID) })
	other, err := s.CreateUser(ctx, "last-submission-other@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser (other): %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, other.ID) })
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID) })

	h := NewChallengesHandler(s)
	r := chi.NewRouter()
	r.With(auth.RequireAuthenticated).Get("/challenges/{id}/submissions/last", h.GetLastSubmission)

	t.Run("no submission yet is 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/challenges/"+ch.ID+"/submissions/last", nil)
		req = req.WithContext(auth.NewContext(req.Context(), auth.Claims{UserID: student.ID, Email: student.Email, Role: "student"}))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404: %s", w.Code, w.Body.String())
		}
	})

	t.Run("unauthenticated is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/challenges/"+ch.ID+"/submissions/last", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401: %s", w.Code, w.Body.String())
		}
	})

	// Two submissions for the student — older in Python, newest in Go — plus
	// one from a different user, to confirm both "newest wins" and "scoped to
	// the caller, not every submitter" in one pass.
	if _, err := s.CreateSubmission(ctx, store.CreateSubmissionRequest{
		ChallengeID: ch.ID, UserID: student.ID, Language: "python", SourceCode: "print('old')",
	}); err != nil {
		t.Fatalf("CreateSubmission (old): %v", err)
	}
	if _, err := s.CreateSubmission(ctx, store.CreateSubmissionRequest{
		ChallengeID: ch.ID, UserID: other.ID, Language: "javascript", SourceCode: "console.log('other')",
	}); err != nil {
		t.Fatalf("CreateSubmission (other user): %v", err)
	}
	if _, err := s.CreateSubmission(ctx, store.CreateSubmissionRequest{
		ChallengeID: ch.ID, UserID: student.ID, Language: "go", SourceCode: "package main\nfunc main() {}",
	}); err != nil {
		t.Fatalf("CreateSubmission (newest): %v", err)
	}

	t.Run("returns the caller's newest submission", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/challenges/"+ch.ID+"/submissions/last", nil)
		req = req.WithContext(auth.NewContext(req.Context(), auth.Claims{UserID: student.ID, Email: student.Email, Role: "student"}))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
		}
		var got store.LastSubmissionCode
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if got.Language != "go" || got.SourceCode != "package main\nfunc main() {}" {
			t.Errorf("got %+v, want the student's newest (go) submission, not the other user's or an older one", got)
		}
	})
}
