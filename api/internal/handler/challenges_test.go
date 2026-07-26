package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/caze/ascend/api/internal/store"
)

// Registers the management routes directly (without auth middleware) to
// exercise handler validation; role enforcement is covered in the auth package.
func newChallengeTestRouter(s *store.Store) chi.Router {
	r := chi.NewRouter()
	ch := NewChallengesHandler(s)
	r.Route("/challenges", func(r chi.Router) {
		ch.Routes(r)
		r.Post("/", ch.Create)
		r.Post("/{id}/test-cases", ch.CreateTestCase)
	})
	return r
}

func TestCreateChallenge_BadJSON(t *testing.T) {
	r := newChallengeTestRouter(nil)
	req := httptest.NewRequest(http.MethodPost, "/challenges", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad JSON: expected 400, got %d", w.Code)
	}
}

func TestCreateChallenge_MissingTitle(t *testing.T) {
	r := newChallengeTestRouter(nil)
	body := `{"slug":"slug-a","difficulty":"easy"}`
	req := httptest.NewRequest(http.MethodPost, "/challenges", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing title: expected 422, got %d", w.Code)
	}
}

func TestCreateChallenge_MissingSlug(t *testing.T) {
	r := newChallengeTestRouter(nil)
	body := `{"title":"My Challenge","difficulty":"medium"}`
	req := httptest.NewRequest(http.MethodPost, "/challenges", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing slug: expected 422, got %d", w.Code)
	}
}

func TestCreateChallenge_MissingDifficulty(t *testing.T) {
	r := newChallengeTestRouter(nil)
	body := `{"slug":"slug-b","title":"My Challenge"}`
	req := httptest.NewRequest(http.MethodPost, "/challenges", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing difficulty: expected 422, got %d", w.Code)
	}
}

func TestCreateChallenge_InvalidDifficulty(t *testing.T) {
	r := newChallengeTestRouter(nil)
	body := `{"slug":"slug-c","title":"My Challenge","difficulty":"extreme"}`
	req := httptest.NewRequest(http.MethodPost, "/challenges", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid difficulty: expected 422, got %d", w.Code)
	}
}

func TestCreateTestCase_MissingExpectedOutput(t *testing.T) {
	r := newChallengeTestRouter(nil)
	body := `{"input":"1 2"}`
	req := httptest.NewRequest(http.MethodPost, "/challenges/00000000-0000-0000-0000-000000000001/test-cases",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing expected_output: expected 422, got %d", w.Code)
	}
}
