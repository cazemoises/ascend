//go:build integration

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

func newChallengeCollectionsRouter(s *store.Store) chi.Router {
	h := NewChallengeCollectionsHandler(s)
	r := chi.NewRouter()
	r.With(auth.RequireAuthenticated, auth.RequireRole("teacher")).Post("/challenge-collections", h.Create)
	r.With(auth.RequireAuthenticated, auth.RequireRole("teacher")).Get("/challenge-collections", h.List)
	return r
}

func TestCreateChallengeCollection_TeacherOnly(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	r := newChallengeCollectionsRouter(s)

	// Only the teacher case actually reaches the handler and inserts a row
	// (student/unauthenticated are rejected by middleware first), so it
	// needs a real user for challenge_collections.teacher_id's FK.
	teacher, err := s.CreateUser(ctx, "collections-teacheronly@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID) })
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM challenge_collections WHERE teacher_id = $1`, teacher.ID) })

	cases := []struct {
		name       string
		claims     *auth.Claims
		wantStatus int
	}{
		{"teacher is allowed", &auth.Claims{UserID: teacher.ID, Email: teacher.Email, Role: "teacher"}, http.StatusCreated},
		{"student is forbidden", &auth.Claims{UserID: "s1", Email: "s@example.com", Role: "student"}, http.StatusForbidden},
		{"unauthenticated is rejected", nil, http.StatusUnauthorized},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/challenge-collections",
				bytes.NewBufferString(`{"title":"Teacher Only Test"}`))
			req.Header.Set("Content-Type", "application/json")
			if tc.claims != nil {
				req = req.WithContext(auth.NewContext(req.Context(), *tc.claims))
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d: %s", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}
