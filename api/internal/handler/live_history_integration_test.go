//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/caze/ascend/api/internal/auth"
	appmw "github.com/caze/ascend/api/internal/middleware"
	"github.com/caze/ascend/api/internal/store"
	"github.com/go-chi/chi/v5"
)

// The same contract runs locally, in PR validation, and against nginx after
// deployment. Only unique disposable fixtures are written; archive reads must
// leave their rooms, rounds and CRDT journal unchanged.
func TestLiveSessionHistory(t *testing.T) {
	for _, mode := range []string{"individual", "timed_challenge", "collaborative"} {
		t.Run(mode, func(t *testing.T) {
			db := openHandlerTestDB(t)
			ctx := context.Background()
			s := store.New(db, nil)
			suffix := fmt.Sprint(time.Now().UnixNano())
			var userIDs, sessionIDs []string
			var listID, challengeID string
			t.Cleanup(func() {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				remove := func(query, id string) {
					if id != "" {
						if _, err := db.ExecContext(cleanupCtx, query, id); err != nil {
							t.Errorf("cleanup fixture: %v", err)
						}
					}
				}
				for _, id := range sessionIDs {
					remove(`DELETE FROM submissions WHERE live_session_id=$1`, id)
					remove(`DELETE FROM live_sessions WHERE id=$1`, id)
				}
				remove(`DELETE FROM problem_lists WHERE id=$1`, listID)
				remove(`DELETE FROM challenges WHERE id=$1`, challengeID)
				for _, id := range userIDs {
					remove(`DELETE FROM users WHERE id=$1`, id)
				}
			})
			user := func(name, role string) store.User {
				t.Helper()
				u, err := s.CreateUser(ctx, "history-"+name+"-"+suffix+"@example.com", "unused")
				if err != nil {
					t.Fatal(err)
				}
				userIDs = append(userIDs, u.ID)
				if _, err := db.ExecContext(ctx, `UPDATE users SET role=$1 WHERE id=$2`, role, u.ID); err != nil {
					t.Fatal(err)
				}
				return u
			}
			teacher, student, peer, stranger, otherTeacher := user("mentor", "teacher"), user("student", "student"), user("peer", "student"), user("stranger", "student"), user("other-mentor", "teacher")
			ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{Slug: "history-" + suffix, Title: "History challenge", Difficulty: "easy"})
			if err != nil {
				t.Fatal(err)
			}
			challengeID = ch.ID
			list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "History " + mode})
			if err != nil {
				t.Fatal(err)
			}
			listID = list.ID
			item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{Title: "First", Difficulty: "easy", LinkedChallengeID: &ch.ID})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{Title: "Never started", Difficulty: "easy", LinkedChallengeID: &ch.ID}); err != nil {
				t.Fatal(err)
			}
			makeSession := func() store.LiveSession {
				t.Helper()
				x, err := s.CreateLiveSessionWithMode(ctx, list.ID, teacher.ID, 1, mode)
				if err != nil {
					t.Fatal(err)
				}
				sessionIDs = append(sessionIDs, x.ID)
				return x
			}
			session, openSession := makeSession(), makeSession()
			for _, id := range []string{student.ID, peer.ID} {
				if _, err := s.JoinLiveSession(ctx, session.ID, id); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "timed_challenge" {
				rounds, err := s.CreateLiveSessionRounds(ctx, session.ID, teacher.ID)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.StartLiveSessionRound(ctx, session.ID, rounds[0].ID, teacher.ID, 60); err != nil {
					t.Fatal(err)
				}
				// An already expired round must still be read without expiration writes.
				if _, err := db.ExecContext(ctx, `UPDATE live_session_rounds SET started_at=now()-interval '2 minutes' WHERE id=$1`, rounds[0].ID); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "collaborative" {
				room, err := s.GetOrCreateLiveRoom(ctx, session.ID, item.ID)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.AppendLiveRoomUpdate(ctx, room.ID, []byte{0, 0}); err != nil {
					t.Fatal(err)
				}
				if _, err := s.FreezeLiveRoom(ctx, room.ID, "all_done"); err != nil {
					t.Fatal(err)
				}
				if _, err := db.ExecContext(ctx, `INSERT INTO submissions(challenge_id,live_session_id,live_room_id,language,source_code,status,passed_count,total_test_cases) VALUES($1,$2,$3,'go','private shared source','accepted',2,2)`, ch.ID, session.ID, room.ID); err != nil {
					t.Fatal(err)
				}
			}
			submit := func(sessionID, userID, status string, testRun bool) {
				t.Helper()
				if _, err := db.ExecContext(ctx, `INSERT INTO submissions(challenge_id,live_session_id,user_id,language,source_code,status,is_test_run) VALUES($1,$2,$3,'go','private individual source',$4,$5)`, ch.ID, sessionID, userID, status, testRun); err != nil {
					t.Fatal(err)
				}
			}
			submit(session.ID, student.ID, "wrong_answer", false)
			submit(session.ID, student.ID, "accepted", false)
			submit(session.ID, student.ID, "accepted", true)
			submit(session.ID, peer.ID, "pending", false)
			submit(openSession.ID, student.ID, "accepted", false)
			if err := s.FinishLiveSession(ctx, session.ID, teacher.ID); err != nil {
				t.Fatal(err)
			}
			h := NewLiveSessionsHandler(s, nil, nil)
			r := chi.NewRouter()
			r.Use(appmw.NewPangolinAuth(s, "").Middleware, appmw.ViewAs, auth.RequireAuthenticated)
			r.Get("/live-sessions", h.List)
			r.Get("/live-sessions/{id}/results", h.Results)
			r.Get("/live-sessions/{id}/dashboard", h.Dashboard)
			server := httptest.NewServer(r)
			defer server.Close()
			base := os.Getenv("LIVE_ROOM_DEPLOYED_URL")
			if base == "" {
				base = server.URL
			}
			client := &http.Client{Timeout: 10 * time.Second}
			request := func(path, email, viewAs string, want int, out any) string {
				t.Helper()
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Remote-Email", email)
				req.Header.Set("X-View-As", viewAs)
				response, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				if response.StatusCode != want {
					t.Fatalf("%s: HTTP %d, want %d", path, response.StatusCode, want)
				}
				if out == nil {
					return ""
				}
				var raw json.RawMessage
				if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(raw, out); err != nil {
					t.Fatal(err)
				}
				return string(raw)
			}
			for _, tc := range []struct {
				u       store.User
				visible bool
			}{{teacher, true}, {student, true}, {peer, true}, {stranger, false}, {otherTeacher, false}} {
				var sessions []store.LiveSession
				request("/live-sessions?status=finished", tc.u.Email, "", http.StatusOK, &sessions)
				found := false
				for _, x := range sessions {
					if x.ID == session.ID {
						found = true
					}
					if x.ID == openSession.ID {
						t.Fatal("open session in history")
					}
				}
				if found != tc.visible {
					t.Fatalf("history visibility=%v, want %v", found, tc.visible)
				}
			}
			var sessions []store.LiveSession
			request("/live-sessions", teacher.Email, "", http.StatusOK, &sessions)
			for _, x := range sessions {
				if x.ID == session.ID {
					t.Fatal("finished session in open listing")
				}
			}
			request("/live-sessions?status=invalid", teacher.Email, "", http.StatusBadRequest, nil)
			path := "/live-sessions/" + session.ID + "/results"
			request(path, "", "", http.StatusUnauthorized, nil)
			request(path, stranger.Email, "", http.StatusNotFound, nil)
			request(path, otherTeacher.Email, "", http.StatusNotFound, nil)
			request(path, teacher.Email, "student", http.StatusNotFound, nil)
			request("/live-sessions/"+openSession.ID+"/results", teacher.Email, "", http.StatusConflict, nil)
			snapshot := func() string {
				t.Helper()
				var value string
				err := db.QueryRowContext(ctx, `SELECT json_build_object('rooms',(SELECT json_agg(r) FROM live_session_rooms r WHERE session_id=$1),'rounds',(SELECT json_agg(r) FROM live_session_rounds r WHERE session_id=$1),'updates',(SELECT json_agg(u) FROM live_room_updates u JOIN live_session_rooms r ON r.id=u.room_id WHERE r.session_id=$1))::text`, session.ID).Scan(&value)
				if err != nil {
					t.Fatal(err)
				}
				return value
			}
			before := snapshot()
			var own, all store.LiveSessionResults
			raw := request(path, student.Email, "", http.StatusOK, &own)
			if strings.Contains(raw, peer.Email) || strings.Contains(raw, peer.ID) || strings.Contains(raw, "private individual source") || strings.Contains(raw, "private shared source") || strings.Contains(raw, "text_snapshot") {
				t.Fatal("student summary exposes peer identity or private code")
			}
			if len(own.Students) != 1 || own.Students[0].UserID != student.ID || own.Students[0].Attempts != 2 || own.Students[0].Accepted != 1 || own.PendingSubmissions != 0 {
				t.Fatalf("wrong own results: %+v", own)
			}
			request(path, teacher.Email, "", http.StatusOK, &all)
			if len(all.Students) != 2 || all.PendingSubmissions != 1 {
				t.Fatalf("wrong owner results: %+v", all)
			}
			if mode == "timed_challenge" && (len(all.Rounds) != 2 || all.Rounds[0].Status != "active" || all.Rounds[1].Status != "pending") {
				t.Fatalf("rounds mutated: %+v", all.Rounds)
			}
			if mode == "collaborative" {
				if len(own.Rooms) != 2 || own.Rooms[0].Verdict == nil || *own.Rooms[0].Verdict != "accepted" || own.Rooms[1].Status != nil {
					t.Fatalf("wrong shared results: %+v", own.Rooms)
				}
				var dashboard store.LiveDashboard
				request("/live-sessions/"+session.ID+"/dashboard", teacher.Email, "", http.StatusOK, &dashboard)
				total := 0
				for _, p := range dashboard.Students {
					total += p.Attempts
				}
				if total != 3 {
					t.Fatalf("collective or test attempt attributed individually: %d", total)
				}
			}
			if after := snapshot(); after != before {
				t.Fatal("historical reads changed rooms, rounds or Yjs journal")
			}
			if mode == "collaborative" {
				// Simulate the window between finishing the session and creating
				// its shared submission, without invoking any finalization writer.
				if _, err := db.ExecContext(ctx, `DELETE FROM submissions WHERE live_session_id=$1 AND live_room_id IS NOT NULL`, session.ID); err != nil {
					t.Fatal(err)
				}
				for _, state := range []string{"open", "frozen"} {
					if _, err := db.ExecContext(ctx, `UPDATE live_session_rooms SET status=$1,text_snapshot='private preparation source' WHERE session_id=$2`, state, session.ID); err != nil {
						t.Fatal(err)
					}
					beforeRead := snapshot()
					request(path, student.Email, "", http.StatusOK, &own)
					if own.PreparingResults != 1 || own.PendingSubmissions != 0 {
						t.Fatalf("preparation gap not reported: %+v", own)
					}
					if snapshot() != beforeRead {
						t.Fatal("preparation read mutated stored data")
					}
				}
				if _, err := db.ExecContext(ctx, `UPDATE live_session_rooms SET text_snapshot=E' \n\t' WHERE session_id=$1`, session.ID); err != nil {
					t.Fatal(err)
				}
				request(path, student.Email, "", http.StatusOK, &own)
				if own.PreparingResults != 0 {
					t.Fatal("empty frozen room must not keep polling forever")
				}
			}
		})
	}
}
