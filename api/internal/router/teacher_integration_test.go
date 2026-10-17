//go:build integration

package router_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	appmw "github.com/caze/ascend/api/internal/middleware"
	"github.com/caze/ascend/api/internal/router"
	"github.com/caze/ascend/api/internal/store"
	_ "github.com/lib/pq"
)

func TestPedagogicalDashboardIdentityAndResolution(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; requires disposable PostgreSQL")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	s := store.New(db, nil)
	createUser := func(email string) store.User {
		t.Helper()
		u, err := s.CreateUser(ctx, email, "unused-test-hash")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, u.ID) })
		return u
	}
	teacher := createUser("dashboard-router-teacher@example.com")
	student := createUser("dashboard-router-student@example.com")
	other := createUser("dashboard-router-other@example.com")
	if _, err := db.ExecContext(ctx, `UPDATE users SET role = 'teacher' WHERE id = $1`, teacher.ID); err != nil {
		t.Fatal(err)
	}
	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{Slug: "dashboard-router", Title: "Dashboard", Difficulty: "easy"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID)
		s.DeleteChallenge(ctx, ch.ID)
	})
	for _, verdict := range []string{"wrong_answer", "runtime_error", "time_limit_exceeded", "pending"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO submissions (challenge_id, user_id, language, source_code, status, created_at) VALUES ($1,$2,'go','private source',$3,'2026-10-12T12:00:00Z')`, ch.ID, student.ID, verdict); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO submissions (challenge_id, user_id, language, source_code, status, is_test_run) VALUES ($1,$2,'go','private test source','accepted',true)`, ch.ID, student.ID); err != nil {
		t.Fatal(err)
	}
	// This is the production router, including trusted-proxy identity and ViewAs.
	r := router.New(s, appmw.NewPangolinAuth(s, ""), nil, nil, nil)
	get := func(email, viewAs string, want int) []store.StudentOverview {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/teacher/students-overview", nil)
		if email != "" {
			req.Header.Set("Remote-Email", email)
		}
		if viewAs != "" {
			req.Header.Set("X-View-As", viewAs)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("status %d, want %d: %s", w.Code, want, w.Body.String())
		}
		if want != http.StatusOK {
			return nil
		}
		var result []store.StudentOverview
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	get("", "", http.StatusUnauthorized)
	get(student.Email, "", http.StatusForbidden)
	get(student.Email, "teacher", http.StatusForbidden)
	get(teacher.Email, "student", http.StatusForbidden)
	check := func(solved bool) {
		t.Helper()
		found := false
		for _, row := range get(teacher.Email, "", http.StatusOK) {
			if row.StudentID == teacher.ID || row.StudentID == other.ID {
				t.Fatal("inactive or teacher account leaked into dashboard")
			}
			if row.StudentID != student.ID {
				continue
			}
			found = true
			wantAttempts, wantUnresolved, wantSolved := 4, 1, 0
			if solved {
				wantAttempts, wantUnresolved, wantSolved = 5, 0, 1
			}
			if row.Stats.TotalSubmissions != wantAttempts || row.Stats.UnresolvedChallenges != wantUnresolved || row.Stats.RecurringDifficulties != wantUnresolved || row.Stats.ChallengesAccepted != wantSolved || row.Stats.WrongAnswerCount != 1 || row.Stats.RuntimeErrorCount != 1 || row.Stats.TimeoutCount != 1 {
				t.Fatalf("unexpected stats: %+v", row.Stats)
			}
			if len(row.WeeklyProgress) != 1 || row.WeeklyProgress[0] != (store.StudentWeek{WeekStart: "2026-10-12", Attempts: wantAttempts, NewlySolved: wantSolved}) {
				t.Fatalf("unexpected weekly JSON: %+v", row.WeeklyProgress)
			}
		}
		if !found {
			t.Fatal("active student missing")
		}
	}
	check(false)
	if _, err := db.ExecContext(ctx, `INSERT INTO submissions (challenge_id, user_id, language, source_code, status, created_at) VALUES ($1,$2,'go','fixed','accepted','2026-10-13T12:00:00Z')`, ch.ID, student.ID); err != nil {
		t.Fatal(err)
	}
	check(true)
}
