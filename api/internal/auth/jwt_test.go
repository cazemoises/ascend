package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

func newTestJWT(t *testing.T, ttl time.Duration) *JWT {
	t.Helper()
	j, err := NewJWT(testSecret, ttl)
	if err != nil {
		t.Fatalf("NewJWT: %v", err)
	}
	return j
}

func TestNewJWT_Validation(t *testing.T) {
	tests := []struct {
		name    string
		secret  []byte
		ttl     time.Duration
		wantErr bool
	}{
		{"valid", testSecret, time.Hour, false},
		{"short secret", []byte("short"), time.Hour, true},
		{"zero ttl", testSecret, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewJWT(tt.secret, tt.ttl)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewJWT err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignVerify_Roundtrip(t *testing.T) {
	j := newTestJWT(t, time.Hour)
	now := time.Now()

	token, err := j.Sign("user-1", "a@b.com", "teacher", now)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	claims, err := j.Verify(token, now)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.UserID != "user-1" || claims.Email != "a@b.com" || claims.Role != "teacher" {
		t.Errorf("claims = %+v", claims)
	}
}

func TestVerify_Failures(t *testing.T) {
	j := newTestJWT(t, time.Hour)
	now := time.Now()
	token, err := j.Sign("user-1", "a@b.com", "student", now)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	parts := strings.Split(token, ".")

	other, err := NewJWT([]byte("ffffffffffffffffffffffffffffffff"), time.Hour)
	if err != nil {
		t.Fatalf("NewJWT: %v", err)
	}
	otherToken, err := other.Sign("user-1", "a@b.com", "student", now)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	tests := []struct {
		name    string
		token   string
		at      time.Time
		wantErr error
	}{
		{"malformed", "abc.def", now, ErrInvalidToken},
		{"tampered payload", parts[0] + ".dGFtcGVyZWQ." + parts[2], now, ErrInvalidToken},
		{"wrong secret", otherToken, now, ErrInvalidToken},
		{"expired", token, now.Add(2 * time.Hour), ErrExpiredToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := j.Verify(tt.token, tt.at)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Verify err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestMiddleware(t *testing.T) {
	j := newTestJWT(t, time.Hour)
	token, err := j.Sign("user-1", "a@b.com", "student", time.Now())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	var gotClaims Claims
	var hadClaims bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClaims, hadClaims = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"valid token", "Bearer " + token, http.StatusOK},
		{"missing header", "", http.StatusUnauthorized},
		{"not bearer", "Basic abc", http.StatusUnauthorized},
		{"garbage token", "Bearer garbage", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hadClaims = false
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()
			j.Middleware(next).ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				if !hadClaims || gotClaims.UserID != "user-1" {
					t.Errorf("claims not propagated: had=%v claims=%+v", hadClaims, gotClaims)
				}
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	j := newTestJWT(t, time.Hour)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	protected := j.Middleware(RequireRole("teacher")(next))

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
			token, err := j.Sign("user-1", "a@b.com", tt.role, time.Now())
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("Authorization", "Bearer "+token)
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

func TestHashCheckPassword(t *testing.T) {
	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := CheckPassword(hash, "s3cret-pass"); err != nil {
		t.Errorf("CheckPassword correct: %v", err)
	}
	if err := CheckPassword(hash, "wrong"); err == nil {
		t.Error("CheckPassword wrong password: expected error")
	}
}
