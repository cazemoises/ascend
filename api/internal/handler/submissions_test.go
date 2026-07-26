package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

func newSubmissionTestRouter(s *store.Store) chi.Router {
	r := chi.NewRouter()
	h := NewChallengesHandler(s)
	r.Post("/challenges/{id}/submissions", h.CreateSubmission)
	return r
}

func TestCreateSubmission_BadJSON(t *testing.T) {
	r := newSubmissionTestRouter(nil)
	req := httptest.NewRequest(http.MethodPost,
		"/challenges/00000000-0000-0000-0000-000000000001/submissions",
		bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad JSON: expected 400, got %d", w.Code)
	}
}

func TestCreateSubmission_MissingLanguage(t *testing.T) {
	r := newSubmissionTestRouter(nil)
	body := `{"source_code":"print('hi')"}`
	req := httptest.NewRequest(http.MethodPost,
		"/challenges/00000000-0000-0000-0000-000000000001/submissions",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing language: expected 422, got %d", w.Code)
	}
}

func TestCreateSubmission_InvalidLanguage(t *testing.T) {
	r := newSubmissionTestRouter(nil)
	body := `{"language":"ruby","source_code":"puts 'hi'"}`
	req := httptest.NewRequest(http.MethodPost,
		"/challenges/00000000-0000-0000-0000-000000000001/submissions",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid language: expected 422, got %d", w.Code)
	}
}

func TestCreateSubmission_MissingSourceCode(t *testing.T) {
	r := newSubmissionTestRouter(nil)
	body := `{"language":"go"}`
	req := httptest.NewRequest(http.MethodPost,
		"/challenges/00000000-0000-0000-0000-000000000001/submissions",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing source_code: expected 422, got %d", w.Code)
	}
}

func TestCreateSubmission_NoClaims_Unauthorized(t *testing.T) {
	r := newSubmissionTestRouter(nil)
	body := `{"language":"go","source_code":"package main"}`
	req := httptest.NewRequest(http.MethodPost,
		"/challenges/00000000-0000-0000-0000-000000000001/submissions",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no claims: expected 401, got %d", w.Code)
	}
}

func TestCursor_Roundtrip(t *testing.T) {
	createdAt := time.Date(2026, 7, 3, 12, 0, 0, 123456000, time.UTC)
	id := "00000000-0000-0000-0000-000000000042"

	gotTime, gotID, err := decodeCursor(encodeCursor(createdAt, id))
	if err != nil {
		t.Fatalf("decodeCursor: %v", err)
	}
	if !gotTime.Equal(createdAt) || gotID != id {
		t.Errorf("roundtrip = (%v, %s), want (%v, %s)", gotTime, gotID, createdAt, id)
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		cursor string
	}{
		{"not base64", "!!!"},
		{"no separator", "MTIzNDU"},
		{"bad timestamp", "YWJjLmRlZg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := decodeCursor(tt.cursor); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestListMySubmissions_InvalidCursor(t *testing.T) {
	j, err := auth.NewJWT([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("NewJWT: %v", err)
	}
	token, err := j.Sign("user-1", "a@b.com", "student", time.Now())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	r := chi.NewRouter()
	h := NewChallengesHandler(nil)
	r.With(j.Middleware).Get("/submissions", h.ListMySubmissions)

	req := httptest.NewRequest(http.MethodGet, "/submissions?cursor=%21%21%21", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid cursor: expected 422, got %d", w.Code)
	}
}

func TestListMySubmissions_NoToken_Unauthorized(t *testing.T) {
	j, err := auth.NewJWT([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("NewJWT: %v", err)
	}
	r := chi.NewRouter()
	h := NewChallengesHandler(nil)
	r.With(j.Middleware).Get("/submissions", h.ListMySubmissions)

	req := httptest.NewRequest(http.MethodGet, "/submissions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no token: expected 401, got %d", w.Code)
	}
}
