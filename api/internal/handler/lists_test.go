package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
)

func newListsTestRouter(j *auth.JWT) chi.Router {
	r := chi.NewRouter()
	h := NewListsHandler(nil)
	teacherOnly := auth.RequireRole("teacher")
	r.With(j.Middleware, teacherOnly).Post("/lists", h.Create)
	r.With(j.Middleware).Get("/lists", h.List)
	return r
}

// TestCreateList_StudentForbidden covers the explicit business rule: a
// student can never create a list, regardless of request body — the role
// gate rejects the request in middleware before the handler (or store) ever
// runs, mirroring the real router's teacherOnly group.
func TestCreateList_StudentForbidden(t *testing.T) {
	j, err := auth.NewJWT([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("NewJWT: %v", err)
	}
	r := newListsTestRouter(j)

	token, err := j.Sign("student-1", "student@example.com", "student", time.Now())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	body := `{"title":"Snuck In List"}`
	req := httptest.NewRequest(http.MethodPost, "/lists", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("student POST /lists: expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListLists_NoToken_Unauthorized(t *testing.T) {
	j, err := auth.NewJWT([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("NewJWT: %v", err)
	}
	r := newListsTestRouter(j)

	req := httptest.NewRequest(http.MethodGet, "/lists", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("no token: expected 401, got %d", w.Code)
	}
}
