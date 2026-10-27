package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/lib/pq"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

const remoteEmailHeader = "Remote-Email"

// PangolinAuth establishes request identity from the Remote-Email header
// forwarded by the Pangolin proxy in front of this deployment — there is no
// login form, password, or token. It never rejects a request itself; routes
// that require a signed-in caller wrap with auth.RequireAuthenticated.
type PangolinAuth struct {
	store        *store.Store
	devFakeEmail string
}

// NewPangolinAuth wires the middleware to the user store. devFakeEmail
// simulates the Remote-Email header for local development without Pangolin
// in front — it is only consulted when the header itself is absent, so a
// real header always wins.
func NewPangolinAuth(s *store.Store, devFakeEmail string) *PangolinAuth {
	return &PangolinAuth{store: s, devFakeEmail: normalizeEmail(devFakeEmail)}
}

func (m *PangolinAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := normalizeEmail(r.Header.Get(remoteEmailHeader))
		if email == "" {
			email = m.devFakeEmail
		}
		if email == "" {
			next.ServeHTTP(w, r)
			return
		}

		u, err := m.findOrCreateUser(r.Context(), email)
		if err != nil {
			// PostgreSQL error messages/details may contain personal data. Log only
			// the SQLSTATE and request ID so failures can be correlated safely.
			code := ""
			var pgErr *pq.Error
			if errors.As(err, &pgErr) {
				code = string(pgErr.Code)
			}
			slog.ErrorContext(r.Context(), "identity lookup failed", "sqlstate", code,
				"canceled", errors.Is(err, context.Canceled),
				"deadline_exceeded", errors.Is(err, context.DeadlineExceeded),
				"request_id", chimw.GetReqID(r.Context()))
			writeJSONError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		claims := auth.Claims{UserID: u.ID, Email: u.Email, Role: u.Role, RealRole: u.Role}
		next.ServeHTTP(w, r.WithContext(auth.NewContext(r.Context(), claims)))
	})
}

func (m *PangolinAuth) findOrCreateUser(ctx context.Context, email string) (store.User, error) {
	u, _, err := m.store.GetUserByEmail(ctx, email)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return store.User{}, err
	}

	hash, err := randomPasswordPlaceholder()
	if err != nil {
		return store.User{}, err
	}
	u, err = m.store.CreateUser(ctx, email, hash)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, store.ErrConflict) {
		return store.User{}, err
	}

	// Lost a create race against a concurrent request for the same new
	// email — the row exists now.
	u, _, err = m.store.GetUserByEmail(ctx, email)
	return u, err
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// randomPasswordPlaceholder satisfies the legacy NOT NULL column without doing
// expensive password hashing on concurrent first visits. The ! prefix makes it
// an invalid bcrypt hash, so this trusted-header account cannot use a password.
func randomPasswordPlaceholder() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "!pangolin:" + base64.RawURLEncoding.EncodeToString(buf), nil
}
