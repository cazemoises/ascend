package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

type Claims struct {
	UserID    string `json:"sub"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type JWT struct {
	secret []byte
	ttl    time.Duration
}

func NewJWT(secret []byte, ttl time.Duration) (*JWT, error) {
	if len(secret) < 32 {
		return nil, errors.New("jwt secret must be at least 32 bytes")
	}
	if ttl <= 0 {
		return nil, errors.New("jwt ttl must be positive")
	}
	return &JWT{secret: secret, ttl: ttl}, nil
}

var b64 = base64.RawURLEncoding

const headerJSON = `{"alg":"HS256","typ":"JWT"}`

func (j *JWT) Sign(userID, email, role string, now time.Time) (string, error) {
	payload, err := json.Marshal(Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(j.ttl).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	signingInput := b64.EncodeToString([]byte(headerJSON)) + "." + b64.EncodeToString(payload)
	return signingInput + "." + b64.EncodeToString(j.hmac(signingInput)), nil
}

func (j *JWT) Verify(token string, now time.Time) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	if subtle.ConstantTimeCompare(sig, j.hmac(parts[0]+"."+parts[1])) != 1 {
		return Claims{}, ErrInvalidToken
	}

	headerBytes, err := b64.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var hdr struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &hdr); err != nil || hdr.Alg != "HS256" {
		return Claims{}, ErrInvalidToken
	}

	payload, err := b64.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if now.Unix() >= c.ExpiresAt {
		return Claims{}, ErrExpiredToken
	}
	return c, nil
}

func (j *JWT) hmac(input string) []byte {
	mac := hmac.New(sha256.New, j.secret)
	mac.Write([]byte(input))
	return mac.Sum(nil)
}

type ctxKey struct{}

func (j *JWT) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) {
			unauthorized(w, "missing bearer token")
			return
		}
		claims, err := j.Verify(strings.TrimPrefix(header, prefix), time.Now())
		if err != nil {
			unauthorized(w, "invalid or expired token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims)))
	})
}

// OptionalMiddleware populates claims when a valid bearer token is present
// but lets anonymous requests through untouched; invalid or expired tokens
// are treated as anonymous rather than rejected. Use it on public routes
// whose response is tailored to the viewer.
func (j *JWT) OptionalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if strings.HasPrefix(header, prefix) {
			if claims, err := j.Verify(strings.TrimPrefix(header, prefix), time.Now()); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(Claims)
	return c, ok
}

// RequireRole authorizes requests whose verified claims carry the given role.
// It must run after Middleware, which populates the context.
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
