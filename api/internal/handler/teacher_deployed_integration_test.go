//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/caze/ascend/api/internal/store"
)

// This smoke test uses only disposable fixtures, removed in FK order. The
// official deployment runner supplies its nginx-mounted API URL and database.
func TestPedagogicalDashboardDeployed(t *testing.T) {
	base := os.Getenv("LIVE_ROOM_DEPLOYED_URL")
	if base == "" {
		t.Skip("LIVE_ROOM_DEPLOYED_URL not set")
	}
	db := openHandlerTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s := store.New(db, nil)
	suffix := fmt.Sprint(time.Now().UnixNano())
	var userIDs []string
	var challengeID string
	t.Cleanup(func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if challengeID != "" {
			for _, query := range []string{`DELETE FROM submissions WHERE challenge_id=$1`, `DELETE FROM challenges WHERE id=$1`} {
				if _, err := db.ExecContext(cleanupCtx, query, challengeID); err != nil {
					t.Error(err)
				}
			}
		}
		for _, id := range userIDs {
			if _, err := db.ExecContext(cleanupCtx, `DELETE FROM users WHERE id=$1`, id); err != nil {
				t.Error(err)
			}
		}
	})
	createUser := func(name string) store.User {
		t.Helper()
		u, err := s.CreateUser(ctx, "dashboard-smoke-"+name+"-"+suffix+"@example.com", "unused")
		if err != nil {
			t.Fatal(err)
		}
		userIDs = append(userIDs, u.ID)
		return u
	}
	teacher, student := createUser("teacher"), createUser("student")
	if _, err := db.ExecContext(ctx, `UPDATE users SET role='teacher' WHERE id=$1`, teacher.ID); err != nil {
		t.Fatal(err)
	}
	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{Slug: "dashboard-smoke-" + suffix, Title: "Dashboard smoke", Difficulty: "easy"})
	if err != nil {
		t.Fatal(err)
	}
	challengeID = ch.ID
	for _, verdict := range []string{"wrong_answer", "runtime_error", "time_limit_exceeded"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO submissions (challenge_id,user_id,language,source_code,status,created_at) VALUES ($1,$2,'go','unused',$3,'2026-10-12T12:00:00Z')`, ch.ID, student.ID, verdict); err != nil {
			t.Fatal(err)
		}
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for _, tc := range []struct {
		email, viewAs string
		status        int
	}{
		{"", "", http.StatusUnauthorized},
		{student.Email, "", http.StatusForbidden},
		{teacher.Email, "student", http.StatusForbidden},
		{teacher.Email, "", http.StatusOK},
	} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/teacher/students-overview", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Remote-Email", tc.email)
		req.Header.Set("X-View-As", tc.viewAs)
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != tc.status {
			response.Body.Close()
			t.Fatalf("dashboard HTTP status=%d, want %d", response.StatusCode, tc.status)
		}
		if tc.status == http.StatusOK {
			var rows []store.StudentOverview
			err := json.NewDecoder(response.Body).Decode(&rows)
			response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range rows {
				if row.StudentID != student.ID {
					continue
				}
				found = true
				if row.Stats.RecurringDifficulties != 1 || row.Stats.UnresolvedChallenges != 1 || row.Stats.WrongAnswerCount != 1 || row.Stats.RuntimeErrorCount != 1 || row.Stats.TimeoutCount != 1 || len(row.WeeklyProgress) != 1 || row.WeeklyProgress[0] != (store.StudentWeek{WeekStart: "2026-10-12", Attempts: 3}) {
					t.Fatalf("deployed dashboard indicators mismatch for fixture student")
				}
			}
			if !found {
				t.Fatal("fixture student missing from deployed dashboard")
			}
		} else {
			response.Body.Close()
		}
	}
}
