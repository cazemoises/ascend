//go:build integration

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

func newListsIntegrationRouter(j *auth.JWT, s *store.Store) chi.Router {
	h := NewListsHandler(s)
	r := chi.NewRouter()
	r.With(j.OptionalMiddleware).Get("/lists", h.List)
	r.With(j.OptionalMiddleware).Get("/lists/{id}", h.Get)
	return r
}

// TestGetList_OwningTeacherSeesOwnDraft reproduces the reported regression:
// a teacher creates a list (draft by default) and immediately GETs it back
// with their own bearer token — it must be a 200, not a 404. Anyone else
// (anonymous or a different teacher) must still get 404 for the same draft,
// and publishing makes it visible to everyone again.
func TestGetList_OwningTeacherSeesOwnDraft(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	owner, err := s.CreateUser(ctx, "lists-draft-owner-integ@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, owner.ID) })

	other, err := s.CreateUser(ctx, "lists-draft-other-integ@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser other: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, other.ID) })

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: owner.ID, Title: "Fresh Draft"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, owner.ID) })

	j, err := auth.NewJWT([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("NewJWT: %v", err)
	}
	r := newListsIntegrationRouter(j, s)

	ownerToken, err := j.Sign(owner.ID, owner.Email, "teacher", time.Now())
	if err != nil {
		t.Fatalf("Sign owner: %v", err)
	}
	otherToken, err := j.Sign(other.ID, other.Email, "teacher", time.Now())
	if err != nil {
		t.Fatalf("Sign other: %v", err)
	}

	t.Run("owning teacher sees own draft", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/lists/"+list.ID, nil)
		req.Header.Set("Authorization", "Bearer "+ownerToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("owner GET /lists/%s: expected 200, got %d: %s", list.ID, w.Code, w.Body.String())
		}
	})

	t.Run("anonymous does not see the draft", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/lists/"+list.ID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("anonymous GET /lists/%s: expected 404, got %d: %s", list.ID, w.Code, w.Body.String())
		}
	})

	t.Run("other teacher does not see the draft", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/lists/"+list.ID, nil)
		req.Header.Set("Authorization", "Bearer "+otherToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("other teacher GET /lists/%s: expected 404, got %d: %s", list.ID, w.Code, w.Body.String())
		}
	})

	t.Run("published list is visible to everyone", func(t *testing.T) {
		if _, err := s.UpdateProblemList(ctx, list.ID, owner.ID, store.UpdateProblemListRequest{
			Title: list.Title, Published: true,
		}); err != nil {
			t.Fatalf("publish: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/lists/"+list.ID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("anonymous GET published /lists/%s: expected 200, got %d: %s", list.ID, w.Code, w.Body.String())
		}
	})
}
