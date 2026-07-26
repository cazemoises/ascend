package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestLimiter(t *testing.T, limit int, window time.Duration) (*RateLimiter, *time.Time) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	current := time.Now()
	l := NewRateLimiter(rdb, limit, window, func(r *http.Request) (string, bool) {
		return "ratelimit:test:user-1", true
	})
	l.now = func() time.Time { return current }
	return l, &current
}

func doRequest(l *RateLimiter) int {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	l.Handler(next).ServeHTTP(w, req)
	return w.Code
}

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	l, _ := newTestLimiter(t, 3, time.Minute)
	for i := 0; i < 3; i++ {
		if code := doRequest(l); code != http.StatusAccepted {
			t.Fatalf("request %d: expected 202, got %d", i+1, code)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	l, _ := newTestLimiter(t, 3, time.Minute)
	for i := 0; i < 3; i++ {
		if code := doRequest(l); code != http.StatusAccepted {
			t.Fatalf("warmup request %d: expected 202, got %d", i+1, code)
		}
	}
	if code := doRequest(l); code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", code)
	}
}

func TestRateLimiter_WindowSlides(t *testing.T) {
	l, current := newTestLimiter(t, 2, time.Minute)
	if code := doRequest(l); code != http.StatusAccepted {
		t.Fatalf("first: expected 202, got %d", code)
	}
	if code := doRequest(l); code != http.StatusAccepted {
		t.Fatalf("second: expected 202, got %d", code)
	}
	if code := doRequest(l); code != http.StatusTooManyRequests {
		t.Fatalf("third: expected 429, got %d", code)
	}

	*current = current.Add(61 * time.Second)
	if code := doRequest(l); code != http.StatusAccepted {
		t.Fatalf("after window: expected 202, got %d", code)
	}
}

func TestRateLimiter_MissingKeyUnauthorized(t *testing.T) {
	l, _ := newTestLimiter(t, 2, time.Minute)
	l.keyFn = func(r *http.Request) (string, bool) { return "", false }
	if code := doRequest(l); code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestRateLimiter_FailsOpenWhenRedisDown(t *testing.T) {
	l, _ := newTestLimiter(t, 1, time.Minute)
	l.rdb = redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	if code := doRequest(l); code != http.StatusAccepted {
		t.Fatalf("expected fail-open 202, got %d", code)
	}
}
