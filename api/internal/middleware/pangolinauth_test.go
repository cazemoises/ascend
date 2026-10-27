package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

func TestPangolinAuth_NoIdentity_PassesThroughAnonymous(t *testing.T) {
	pa := NewPangolinAuth(store.New(nil, nil), "")
	var hadClaims bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadClaims = auth.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	pa.Middleware(next).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if hadClaims {
		t.Error("expected no claims for a request with no Remote-Email header and no DEV_FAKE_EMAIL")
	}
}

func TestPasswordPlaceholderCannotAuthenticate(t *testing.T) {
	first, err := randomPasswordPlaceholder()
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomPasswordPlaceholder()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("placeholders must be random per account")
	}
	if _, err := bcrypt.Cost([]byte(first)); err == nil {
		t.Fatal("placeholder must not be a usable bcrypt hash")
	}
}
