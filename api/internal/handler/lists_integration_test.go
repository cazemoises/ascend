//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

func newListsIntegrationRouter(s *store.Store) chi.Router {
	h := NewListsHandler(s)
	r := chi.NewRouter()
	r.Get("/lists", h.List)
	r.Get("/lists/{id}", h.Get)
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

	r := newListsIntegrationRouter(s)

	t.Run("owning teacher sees own draft", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/lists/"+list.ID, nil)
		req = req.WithContext(auth.NewContext(req.Context(), auth.Claims{UserID: owner.ID, Email: owner.Email, Role: "teacher"}))
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
		req = req.WithContext(auth.NewContext(req.Context(), auth.Claims{UserID: other.ID, Email: other.Email, Role: "teacher"}))
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

// TestListLists_MultipleOverlappingCurrentWeeks covers the reported bug's
// first investigation step: is_current is computed independently per list
// in ListsHandler.List (no dedup, no "pick one" logic), so two published
// lists whose date ranges both cover today must both come back
// is_current=true in the same response. This confirms the backend is
// correct — the bug (only one list showing in the frontend's "Semana
// atual" section) is a frontend selection bug, not a backend one.
func TestListLists_MultipleOverlappingCurrentWeeks(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	teacher, err := s.CreateUser(ctx, "lists-multi-current-integ@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID) })

	today := time.Now().UTC()
	startA := today.AddDate(0, 0, -3)
	endA := today.AddDate(0, 0, 10)
	startB := today.AddDate(0, 0, -1)
	endB := today.AddDate(0, 0, 5)

	listA, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{
		TeacherID: teacher.ID, Title: "Overlap A", WeekStart: &startA, WeekEnd: &endA,
	})
	if err != nil {
		t.Fatalf("CreateProblemList A: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, listA.ID, teacher.ID) })
	if _, err := s.UpdateProblemList(ctx, listA.ID, teacher.ID, store.UpdateProblemListRequest{
		Title: listA.Title, WeekStart: &startA, WeekEnd: &endA, Published: true,
	}); err != nil {
		t.Fatalf("publish A: %v", err)
	}

	listB, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{
		TeacherID: teacher.ID, Title: "Overlap B", WeekStart: &startB, WeekEnd: &endB,
	})
	if err != nil {
		t.Fatalf("CreateProblemList B: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, listB.ID, teacher.ID) })
	if _, err := s.UpdateProblemList(ctx, listB.ID, teacher.ID, store.UpdateProblemListRequest{
		Title: listB.Title, WeekStart: &startB, WeekEnd: &endB, Published: true,
	}); err != nil {
		t.Fatalf("publish B: %v", err)
	}

	r := newListsIntegrationRouter(s)

	req := httptest.NewRequest(http.MethodGet, "/lists", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /lists: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []struct {
		ID        string `json:"id"`
		IsCurrent bool   `json:"is_current"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	gotA, gotB := false, false
	for _, item := range resp {
		switch item.ID {
		case listA.ID:
			gotA = item.IsCurrent
		case listB.ID:
			gotB = item.IsCurrent
		}
	}
	if !gotA || !gotB {
		t.Errorf("expected both overlapping lists to be is_current=true, got A=%v B=%v", gotA, gotB)
	}
}
