package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewContext_FromContext_Roundtrip(t *testing.T) {
	claims := Claims{UserID: "user-1", Email: "a@b.com", Role: "teacher"}
	ctx := NewContext(t.Context(), claims)

	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("expected claims present")
	}
	if got != claims {
		t.Errorf("claims = %+v, want %+v", got, claims)
	}
}

func TestFromContext_Absent(t *testing.T) {
	if _, ok := FromContext(t.Context()); ok {
		t.Error("expected no claims in bare context")
	}
}

func TestRequireAuthenticated(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	protected := RequireAuthenticated(next)

	t.Run("with claims", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(NewContext(req.Context(), Claims{UserID: "user-1"}))
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})

	t.Run("without claims", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})
}

func TestRequireRole(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	protected := RequireRole("teacher")(next)

	tests := []struct {
		name       string
		role       string
		wantStatus int
	}{
		{"teacher allowed", "teacher", http.StatusOK},
		{"student forbidden", "student", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req = req.WithContext(NewContext(req.Context(), Claims{UserID: "user-1", Role: tt.role}))
			w := httptest.NewRecorder()
			protected.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}

	t.Run("no claims unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		w := httptest.NewRecorder()
		RequireRole("teacher")(next).ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})
}
