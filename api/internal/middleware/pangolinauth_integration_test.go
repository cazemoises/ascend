//go:build integration

package middleware

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func claimsFromRequest(t *testing.T, pa *PangolinAuth, req *http.Request) (auth.Claims, bool, int) {
	t.Helper()
	var got auth.Claims
	var ok bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = auth.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	w := httptest.NewRecorder()
	pa.Middleware(next).ServeHTTP(w, req)
	return got, ok, w.Code
}

func TestPangolinAuth_ExistingUser(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := t.Context()

	email := "pangolin-existing@example.com"
	// Defensive pre-clean: t.Cleanup below only registers after CreateUser
	// succeeds, so a previous run interrupted before that point (process
	// killed mid-test, no chance for cleanup to ever run) leaves this row
	// behind — and every subsequent run then fails CreateUser outright on
	// the unique email index, with no id to clean up with. Self-heal here
	// instead of requiring a one-off manual DB fix.
	db.ExecContext(ctx, `DELETE FROM users WHERE email = $1`, email)

	hash, err := auth.HashPassword("unused")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	u, err := s.CreateUser(ctx, email, hash)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, u.ID) })

	pa := NewPangolinAuth(s, "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Remote-Email", "Pangolin-Existing@example.com")

	got, ok, status := claimsFromRequest(t, pa, req)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if !ok || got.UserID != u.ID || got.Email != u.Email || got.Role != u.Role {
		t.Errorf("claims = %+v, want id=%s email=%s role=%s", got, u.ID, u.Email, u.Role)
	}
}

func TestPangolinAuth_NewEmail_CreatesStudent(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := t.Context()
	email := "pangolin-brand-new@example.com"
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE email = $1`, email) })

	pa := NewPangolinAuth(s, "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Remote-Email", email)

	got, ok, status := claimsFromRequest(t, pa, req)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if !ok || got.Email != email || got.Role != "student" || got.UserID == "" {
		t.Errorf("claims = %+v, want a new student for %s", got, email)
	}

	// Idempotent: a second request for the same email resolves to the same row,
	// not a fresh insert (which would 409 on the unique email index).
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Remote-Email", email)
	got2, ok2, status2 := claimsFromRequest(t, pa, req2)
	if status2 != http.StatusOK || !ok2 || got2.UserID != got.UserID {
		t.Errorf("second lookup = %+v, want same user id %s", got2, got.UserID)
	}
}

func TestPangolinAuth_DevFakeEmail_OnlyWhenHeaderAbsent(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := t.Context()

	devEmail := "pangolin-dev-fallback@example.com"
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE email = $1`, devEmail) })

	headerEmail := "pangolin-header-wins@example.com"
	// Same defensive pre-clean as TestPangolinAuth_ExistingUser above — see
	// that comment for why cleanup-after-success alone can't self-heal a
	// leftover row from an interrupted previous run.
	db.ExecContext(ctx, `DELETE FROM users WHERE email = $1`, headerEmail)

	headerHash, err := auth.HashPassword("unused")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	headerUser, err := s.CreateUser(ctx, headerEmail, headerHash)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, headerUser.ID) })

	pa := NewPangolinAuth(s, devEmail)

	t.Run("no header falls back to dev email", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		got, ok, status := claimsFromRequest(t, pa, req)
		if status != http.StatusOK || !ok || got.Email != devEmail {
			t.Errorf("claims = %+v, want dev fallback email %s", got, devEmail)
		}
	})

	t.Run("header present takes priority over dev email", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Remote-Email", headerUser.Email)
		got, ok, status := claimsFromRequest(t, pa, req)
		if status != http.StatusOK || !ok || got.UserID != headerUser.ID {
			t.Errorf("claims = %+v, want header user %s, not dev email", got, headerUser.ID)
		}
	})
}
