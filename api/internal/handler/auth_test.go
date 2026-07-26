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

func newAuthTestRouter(t *testing.T) chi.Router {
	t.Helper()
	j, err := auth.NewJWT([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatalf("NewJWT: %v", err)
	}
	r := chi.NewRouter()
	r.Route("/auth", NewAuthHandler(nil, j).Routes)
	return r
}

func TestAuth_Validation(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		wantStatus int
	}{
		{"register bad JSON", "/auth/register", "not-json", http.StatusBadRequest},
		{"register invalid email", "/auth/register", `{"email":"nope","password":"12345678"}`, http.StatusUnprocessableEntity},
		{"register missing email", "/auth/register", `{"password":"12345678"}`, http.StatusUnprocessableEntity},
		{"register short password", "/auth/register", `{"email":"a@b.com","password":"short"}`, http.StatusUnprocessableEntity},
		{"login bad JSON", "/auth/login", "not-json", http.StatusBadRequest},
		{"login missing email", "/auth/login", `{"password":"12345678"}`, http.StatusUnprocessableEntity},
		{"login missing password", "/auth/login", `{"email":"a@b.com"}`, http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newAuthTestRouter(t)
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("%s: expected %d, got %d", tt.name, tt.wantStatus, w.Code)
			}
		})
	}
}
