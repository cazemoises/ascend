package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caze/ascend/api/internal/auth"
)

func TestMe_Authenticated(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req = req.WithContext(auth.NewContext(req.Context(), auth.Claims{
		UserID: "user-1", Email: "a@b.com", Role: "teacher",
	}))
	w := httptest.NewRecorder()
	Me(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var got MeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := MeResponse{ID: "user-1", Email: "a@b.com", Role: "teacher"}
	if got != want {
		t.Errorf("body = %+v, want %+v", got, want)
	}
}

func TestMe_Unauthenticated(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()
	Me(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
