package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
)

func newListsTestRouter() chi.Router {
	r := chi.NewRouter()
	h := NewListsHandler(nil)
	teacherOnly := auth.RequireRole("teacher")
	r.With(auth.RequireAuthenticated, teacherOnly).Post("/lists", h.Create)
	r.With(auth.RequireAuthenticated, teacherOnly).Post("/lists/import", h.Import)
	r.Get("/lists", h.List)
	return r
}

// withClaims simulates an identity already resolved by
// middleware.PangolinAuth — real requests never carry a bearer token, so
// tests inject auth.Claims directly into the request context instead.
func withClaims(req *http.Request, claims auth.Claims) *http.Request {
	return req.WithContext(auth.NewContext(req.Context(), claims))
}

// TestCreateList_StudentForbidden covers the explicit business rule: a
// student can never create a list, regardless of request body — the role
// gate rejects the request in middleware before the handler (or store) ever
// runs, mirroring the real router's teacherOnly group.
func TestCreateList_StudentForbidden(t *testing.T) {
	r := newListsTestRouter()

	body := `{"title":"Snuck In List"}`
	req := httptest.NewRequest(http.MethodPost, "/lists", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withClaims(req, auth.Claims{UserID: "student-1", Email: "student@example.com", Role: "student"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("student POST /lists: expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestImportList_StudentForbidden mirrors TestCreateList_StudentForbidden
// for the import endpoint: the role gate rejects the request before the
// handler (or store) ever runs.
func TestImportList_StudentForbidden(t *testing.T) {
	r := newListsTestRouter()

	body := `{"title":"Snuck In List","items":[]}`
	req := httptest.NewRequest(http.MethodPost, "/lists/import", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withClaims(req, auth.Claims{UserID: "student-1", Email: "student@example.com", Role: "student"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("student POST /lists/import: expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestImportList_InvalidItemDifficulty_UnprocessableEntity covers the
// validation path that never reaches the store: an invalid item difficulty
// 422s with a message identifying which item is bad, before any transaction
// starts.
func TestImportList_InvalidItemDifficulty_UnprocessableEntity(t *testing.T) {
	r := newListsTestRouter()

	body := `{"title":"Bancos de dados","items":[
		{"title":"Item 1","difficulty":"easy","body":"ok"},
		{"title":"Item 2","difficulty":"impossible","body":"bad"}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/lists/import", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withClaims(req, auth.Claims{UserID: "teacher-1", Email: "teacher@example.com", Role: "teacher"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("item 1")) {
		t.Errorf("expected error message to identify item index 1, got: %s", w.Body.String())
	}
}

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(dateOnlyLayout, s)
	if err != nil {
		t.Fatalf("mustDate(%q): %v", s, err)
	}
	return d
}

func TestIsCurrentWeek(t *testing.T) {
	now := mustDate(t, "2026-07-23") // Thursday, inside the 21-27 range below

	tests := []struct {
		name  string
		start *time.Time
		end   *time.Time
		want  bool
	}{
		{
			name:  "today inside range",
			start: new(mustDate(t, "2026-07-21")),
			end:   new(mustDate(t, "2026-07-27")),
			want:  true,
		},
		{
			name:  "today equals start boundary",
			start: new(now),
			end:   new(mustDate(t, "2026-07-27")),
			want:  true,
		},
		{
			name:  "today equals end boundary",
			start: new(mustDate(t, "2026-07-21")),
			end:   new(now),
			want:  true,
		},
		{
			name:  "today before range",
			start: new(mustDate(t, "2026-07-24")),
			end:   new(mustDate(t, "2026-07-30")),
			want:  false,
		},
		{
			name:  "today after range",
			start: new(mustDate(t, "2026-07-01")),
			end:   new(mustDate(t, "2026-07-22")),
			want:  false,
		},
		{
			name:  "week_start nil",
			start: nil,
			end:   new(mustDate(t, "2026-07-27")),
			want:  false,
		},
		{
			name:  "week_end nil",
			start: new(mustDate(t, "2026-07-21")),
			end:   nil,
			want:  false,
		},
		{
			name:  "both nil",
			start: nil,
			end:   nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCurrentWeek(tt.start, tt.end, now); got != tt.want {
				t.Errorf("isCurrentWeek() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUpcomingWeek(t *testing.T) {
	now := mustDate(t, "2026-07-23")

	tests := []struct {
		name  string
		start *time.Time
		want  bool
	}{
		{
			name:  "tomorrow",
			start: new(mustDate(t, "2026-07-24")),
			want:  true,
		},
		{
			name:  "further in the future",
			start: new(mustDate(t, "2026-08-01")),
			want:  true,
		},
		{
			name:  "today",
			start: new(now),
			want:  false,
		},
		{
			name:  "past",
			start: new(mustDate(t, "2026-07-22")),
			want:  false,
		},
		{
			name:  "week_start nil",
			start: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUpcomingWeek(tt.start, now); got != tt.want {
				t.Errorf("isUpcomingWeek() = %v, want %v", got, tt.want)
			}
		})
	}
}
