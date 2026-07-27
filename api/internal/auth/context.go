package auth

import (
	"context"
	"encoding/json"
	"net/http"
)

// Claims describes the identity attached to a request context. It is
// populated by middleware.PangolinAuth (from the Remote-Email header, or
// DEV_FAKE_EMAIL in local dev) — there is no token to sign or verify.
type Claims struct {
	UserID string
	Email  string
	Role   string
}

type ctxKey struct{}

// NewContext returns a copy of ctx carrying claims, retrievable via
// FromContext. Used by middleware.PangolinAuth to populate the request
// context after resolving identity from the Remote-Email header.
func NewContext(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, claims)
}

func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(Claims)
	return c, ok
}

// RequireAuthenticated rejects requests whose context carries no claims —
// i.e. no Remote-Email header (and no DEV_FAKE_EMAIL) was present upstream.
func RequireAuthenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := FromContext(r.Context()); !ok {
			unauthorized(w, "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole authorizes requests whose verified claims carry the given role.
// It must run after RequireAuthenticated (or any middleware populating the
// context), which populates the context.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := FromContext(r.Context())
			if !ok {
				unauthorized(w, "authentication required")
				return
			}
			if claims.Role != role {
				writeStatus(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func unauthorized(w http.ResponseWriter, msg string) {
	writeStatus(w, http.StatusUnauthorized, msg)
}

func writeStatus(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
