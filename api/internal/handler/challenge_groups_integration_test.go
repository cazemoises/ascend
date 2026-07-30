//go:build integration

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

func newChallengeGroupsRouter(s *store.Store) chi.Router {
	h := NewChallengeGroupsHandler(s)
	r := chi.NewRouter()
	r.With(auth.RequireAuthenticated, auth.RequireRole("teacher")).Post("/challenge-groups", h.Create)
	r.With(auth.RequireAuthenticated, auth.RequireRole("teacher")).Get("/challenge-groups", h.List)
	r.With(auth.RequireAuthenticated, auth.RequireRole("teacher")).Patch("/challenge-groups/reorder", h.Reorder)
	return r
}

func TestCreateChallengeGroup_TeacherOnly(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	r := newChallengeGroupsRouter(s)

	teacher, err := s.CreateUser(ctx, "groups-teacheronly@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID) })
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM challenge_groups WHERE teacher_id = $1`, teacher.ID) })

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
			req := httptest.NewRequest(http.MethodPost, "/challenge-groups",
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

func TestReorderChallengeGroups_Handler(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	r := newChallengeGroupsRouter(s)

	teacher, err := s.CreateUser(ctx, "groups-reorder-handler@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID) })
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM challenge_groups WHERE teacher_id = $1`, teacher.ID) })

	first, err := s.CreateChallengeGroup(ctx, store.CreateChallengeGroupRequest{
		TeacherID: teacher.ID, Title: "First",
	})
	if err != nil {
		t.Fatalf("CreateChallengeGroup (first): %v", err)
	}
	second, err := s.CreateChallengeGroup(ctx, store.CreateChallengeGroupRequest{
		TeacherID: teacher.ID, Title: "Second",
	})
	if err != nil {
		t.Fatalf("CreateChallengeGroup (second): %v", err)
	}

	teacherClaims := auth.Claims{UserID: teacher.ID, Email: teacher.Email, Role: "teacher"}
	studentClaims := auth.Claims{UserID: "s1", Email: "s@example.com", Role: "student"}

	cases := []struct {
		name       string
		claims     *auth.Claims
		wantStatus int
	}{
		{"student is forbidden", &studentClaims, http.StatusForbidden},
		{"unauthenticated is rejected", nil, http.StatusUnauthorized},
		{"teacher is allowed", &teacherClaims, http.StatusNoContent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"items":[{"id":"` + first.ID + `","ordinal":` + strconv.Itoa(second.Ordinal) + `},` +
				`{"id":"` + second.ID + `","ordinal":` + strconv.Itoa(first.Ordinal) + `}]}`
			req := httptest.NewRequest(http.MethodPatch, "/challenge-groups/reorder", bytes.NewBufferString(body))
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
