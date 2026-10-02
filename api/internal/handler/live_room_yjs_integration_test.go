//go:build integration

package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

// Runs the actual frontend transport and Yjs against two real API handlers and
// PostgreSQL. The second instance deliberately has no Redis subscriber: journal
// catch-up must repair missed broadcasts, not merely late joins.
func TestLiveRoomYjsConvergence(t *testing.T) {
	node := os.Getenv("LIVE_ROOM_NODE")
	if node == "" {
		var err error
		node, err = exec.LookPath("node")
		if err != nil {
			t.Skip("Node 24 required; set LIVE_ROOM_NODE for cross-platform integration")
		}
	}
	db := openHandlerTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var rdb *redis.Client
	if address := os.Getenv("REDIS_URL"); address != "" {
		options, err := redis.ParseURL(address)
		if err != nil {
			t.Fatal(err)
		}
		rdb = redis.NewClient(options)
		defer rdb.Close()
		if err := rdb.Ping(ctx).Err(); err != nil {
			t.Fatal(err)
		}
	}
	s := store.New(db, rdb)
	suffix := fmt.Sprint(time.Now().UnixNano())
	var sessionID, listID, challengeID string
	userIDs := []string{}
	// A single cleanup preserves FK ordering even if setup fails partway.
	t.Cleanup(func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		remove := func(query, id string) {
			if id == "" {
				return
			}
			if _, err := db.ExecContext(cleanupCtx, query, id); err != nil {
				t.Errorf("cleanup fixture: %v", err)
			}
		}
		remove(`DELETE FROM live_sessions WHERE id=$1`, sessionID)
		remove(`DELETE FROM problem_lists WHERE id=$1`, listID)
		remove(`DELETE FROM challenges WHERE id=$1`, challengeID)
		for _, id := range userIDs {
			remove(`DELETE FROM users WHERE id=$1`, id)
		}
	})
	teacher, err := s.CreateUser(ctx, "room-yjs-teacher-"+suffix+"@example.com", "unused")
	if err != nil {
		t.Fatal(err)
	}
	userIDs = append(userIDs, teacher.ID)
	if _, err := db.Exec(`UPDATE users SET role='teacher' WHERE id=$1`, teacher.ID); err != nil {
		t.Fatal(err)
	}
	challenge, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{Slug: "room-yjs-regression-" + suffix, Title: "Yjs regression", Difficulty: "easy"})
	if err != nil {
		t.Fatal(err)
	}
	challengeID = challenge.ID
	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Yjs regression"})
	if err != nil {
		t.Fatal(err)
	}
	listID = list.ID
	item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{Title: "Shared code", Difficulty: "easy", LinkedChallengeID: &challenge.ID})
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.CreateLiveSessionWithMode(ctx, list.ID, teacher.ID, 30, "collaborative")
	if err != nil {
		t.Fatal(err)
	}
	sessionID = session.ID
	claims := []auth.Claims{{UserID: teacher.ID, Email: teacher.Email, Role: "teacher", RealRole: "teacher"}}
	for _, name := range []string{"a", "b", "c"} {
		email := "room-yjs-" + name + "-" + suffix + "@example.com"
		student, err := s.CreateUser(ctx, email, "unused")
		if err != nil {
			t.Fatal(err)
		}
		userIDs = append(userIDs, student.ID)
		if _, err := s.JoinLiveSession(ctx, session.ID, student.ID); err != nil {
			t.Fatal(err)
		}
		claims = append(claims, auth.Claims{UserID: student.ID, Email: student.Email, Role: "student", RealRole: "student"})
	}
	hubs := []*RoomHub{NewRoomHub(), NewRoomHub()}
	if rdb != nil {
		go hubs[0].Run(ctx, rdb)
	}
	handlers := []*LiveSessionsHandler{NewLiveSessionsHandler(s, nil, hubs[0]), NewLiveSessionsHandler(s, nil, hubs[1])}
	router := chi.NewRouter()
	router.Get("/{id}/rooms/{list_item_id}/ws/{client}", func(w http.ResponseWriter, r *http.Request) {
		index := int(chi.URLParam(r, "client")[0] - '0')
		if index < 0 || index >= len(claims) {
			http.NotFound(w, r)
			return
		}
		handlers[index%2].RoomWS(w, r.WithContext(auth.NewContext(r.Context(), claims[index])))
	})
	server := httptest.NewServer(router)
	defer server.Close()
	_, source, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(source), "../../..", "web/tests/live-room-integration.mjs")
	if strings.HasSuffix(node, ".exe") {
		converted, err := exec.Command("wslpath", "-w", script).Output()
		if err != nil {
			t.Fatal(err)
		}
		script = strings.TrimSpace(string(converted))
	}
	address := "ws" + strings.TrimPrefix(server.URL, "http") + "/" + session.ID + "/rooms/" + item.ID + "/ws/"
	deployed := os.Getenv("LIVE_ROOM_DEPLOYED_URL")
	if deployed != "" {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, deployed+"/live-sessions/"+session.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Remote-Email", teacher.Email)
		response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("deployed Live Session detail: status %d", response.StatusCode)
		}
		address = "ws" + strings.TrimPrefix(deployed, "http") + "/live-sessions/" + session.ID + "/rooms/" + item.ID + "/ws"
	}
	command := exec.CommandContext(ctx, node, script, address)
	if deployed != "" {
		emails := []string{}
		for _, claim := range claims {
			emails = append(emails, claim.Email)
		}
		command.Env = append(os.Environ(), "LIVE_ROOM_EMAILS="+strings.Join(emails, ","))
	}
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatalf("Yjs convergence: %v", err)
	}
	for _, hub := range hubs {
		room, err := s.GetOrCreateLiveRoom(ctx, session.ID, item.ID)
		if err == nil {
			hub.Close(room.ID)
		}
	}
}
