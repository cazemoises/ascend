//go:build integration

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

// newTeacherOverviewRouter mirrors exactly how router.go wires this route —
// RequireAuthenticated + RequireRole("teacher") — so the test exercises the
// same guard that's actually in front of the endpoint in production.
func newTeacherOverviewRouter(s *store.Store) chi.Router {
	th := NewTeacherHandler(s)
	r := chi.NewRouter()
	r.With(auth.RequireAuthenticated, auth.RequireRole("teacher")).
		Get("/teacher/students-overview", th.StudentsOverview)
	return r
}

func TestStudentsOverview_TeacherOnly(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	r := newTeacherOverviewRouter(s)

	cases := []struct {
		name       string
		claims     *auth.Claims
		wantStatus int
	}{
		{"teacher is allowed", &auth.Claims{UserID: "t1", Email: "t@example.com", Role: "teacher"}, http.StatusOK},
		{"student is forbidden", &auth.Claims{UserID: "s1", Email: "s@example.com", Role: "student"}, http.StatusForbidden},
		{"unauthenticated is rejected", nil, http.StatusUnauthorized},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/teacher/students-overview", nil)
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
