package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/caze/ascend/api/internal/auth"
)

// KeyFunc extracts the rate-limit bucket key from a request. The boolean is
// false when no key can be derived (request is rejected as unauthorized).
type KeyFunc func(r *http.Request) (string, bool)

// UserKey buckets requests by the authenticated user set by PangolinAuth.
func UserKey(r *http.Request) (string, bool) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		return "", false
	}
	return "ratelimit:submissions:" + claims.UserID, true
}

type RateLimiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
	keyFn  KeyFunc
	now    func() time.Time
}

func NewRateLimiter(rdb *redis.Client, limit int, window time.Duration, keyFn KeyFunc) *RateLimiter {
	return &RateLimiter{rdb: rdb, limit: limit, window: window, keyFn: keyFn, now: time.Now}
}

// Handler enforces a sliding-window limit backed by a Redis sorted set: one
// member per request scored by unix-nano timestamp. Entries older than the
// window are pruned on every request. Fails open if Redis is unavailable so
// an outage does not block submissions.
func (l *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, ok := l.keyFn(r)
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		now := l.now()
		windowStart := now.Add(-l.window)
		member, err := randomMember(now)
		if err != nil {
			slog.WarnContext(r.Context(), "rate limiter degraded, allowing request", "err", err)
			next.ServeHTTP(w, r)
			return
		}

		pipe := l.rdb.TxPipeline()
		pipe.ZRemRangeByScore(r.Context(), key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))
		pipe.ZAdd(r.Context(), key, redis.Z{Score: float64(now.UnixNano()), Member: member})
		countCmd := pipe.ZCard(r.Context(), key)
		pipe.Expire(r.Context(), key, l.window)
		if _, err := pipe.Exec(r.Context()); err != nil {
			slog.WarnContext(r.Context(), "rate limiter degraded, allowing request", "err", err)
			next.ServeHTTP(w, r)
			return
		}

		if countCmd.Val() > int64(l.limit) {
			if err := l.rdb.ZRem(r.Context(), key, member).Err(); err != nil {
				slog.WarnContext(r.Context(), "rate limiter cleanup failed", "err", err)
			}
			w.Header().Set("Retry-After", strconv.Itoa(l.retryAfterSeconds(r, key, now)))
			writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// retryAfterSeconds derives the wait from the oldest entry still inside the
// window; falls back to the full window if the lookup fails.
func (l *RateLimiter) retryAfterSeconds(r *http.Request, key string, now time.Time) int {
	entries, err := l.rdb.ZRangeWithScores(r.Context(), key, 0, 0).Result()
	if err != nil || len(entries) == 0 {
		return int(l.window.Seconds())
	}
	oldest := time.Unix(0, int64(entries[0].Score))
	wait := l.window - now.Sub(oldest)
	if wait < time.Second {
		return 1
	}
	return int(wait.Round(time.Second).Seconds())
}

func randomMember(now time.Time) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate rate limit member: %w", err)
	}
	return strconv.FormatInt(now.UnixNano(), 10) + "-" + hex.EncodeToString(buf), nil
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
