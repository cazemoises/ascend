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

func newLinkedItemRouter(s *store.Store) chi.Router {
	h := NewListsHandler(s)
	r := chi.NewRouter()
	r.With(auth.RequireAuthenticated).Post("/list-items/{id}/complete", h.CompleteItem)
	r.With(auth.RequireAuthenticated).Delete("/list-items/{id}/complete", h.UncompleteItem)
	return r
}

// TestCompleteItem_RejectsLinkedItem covers the actual HTTP contract: a
// student POSTing /complete on an item that's linked to a challenge gets a
// 422 with a message telling them to solve the challenge instead — not a
// silent no-op and not a 500.
func TestCompleteItem_RejectsLinkedItem(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	r := newLinkedItemRouter(s)

	teacher, err := s.CreateUser(ctx, "linked-complete-teacher@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser (teacher): %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID) })

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "linked-complete-challenge", Title: "Linked Complete Challenge", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Linked Complete"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })
	if _, err := s.UpdateProblemList(ctx, list.ID, teacher.ID, store.UpdateProblemListRequest{Title: list.Title, Published: true}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
		Title: "Solve it", Difficulty: "easy", Body: "b", LinkedChallengeID: &ch.ID,
	})
	if err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/list-items/"+item.ID+"/complete", nil)
	req = req.WithContext(auth.NewContext(req.Context(),
		auth.Claims{UserID: "student-1", Email: "student@example.com", Role: "student"}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected a non-empty error message explaining the item is linked to a challenge")
	}
}
