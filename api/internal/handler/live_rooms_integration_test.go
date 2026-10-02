//go:build integration

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

// TestRoomWSTeacherWritesWithoutJoiningRoom protects the distinction between
// collaborative editing and room presence: a real teacher can send Yjs
// snapshots while remaining absent from live_room_participants.
func TestRoomWSTeacherWritesWithoutJoiningRoomAndFreezeReplaysMissedUpdate(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	teacher, err := s.CreateUser(ctx, "live-room-ws-teacher@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser teacher: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID) })
	lang := "python"
	challenge, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "live-room-ws-teacher-challenge", Title: "WebSocket Challenge", Difficulty: "easy", Language: &lang,
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteChallenge(ctx, challenge.ID) })

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "WebSocket List"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteProblemList(ctx, list.ID, teacher.ID) })

	item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
		Title: "Item", Difficulty: "easy", LinkedChallengeID: &challenge.ID,
	})
	if err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}
	session, err := s.CreateLiveSessionWithMode(ctx, list.ID, teacher.ID, 1, "collaborative")
	if err != nil {
		t.Fatalf("CreateLiveSessionWithMode: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM live_sessions WHERE id = $1`, session.ID) })

	h := NewLiveSessionsHandler(s, nil, nil)
	asTeacher := auth.Claims{UserID: teacher.ID, Email: teacher.Email, Role: "student", RealRole: "teacher"}
	roomWSFinished := make(chan struct{})
	r := chi.NewRouter()
	r.Get("/live-sessions/{id}/rooms/{list_item_id}/ws", func(w http.ResponseWriter, r *http.Request) {
		defer close(roomWSFinished)
		h.RoomWS(w, r.WithContext(auth.NewContext(r.Context(), asTeacher)))
	})
	server := httptest.NewServer(r)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/live-sessions/" + session.ID + "/rooms/" + item.ID + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial RoomWS: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, append([]byte{roomFrameUpdate}, []byte("teacher-yjs-state")...)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	// Wait for the persisted echo, then bypass broadcast to simulate a missed
	// Redis event immediately followed by a freeze notification.
	for {
		_, frame, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if len(frame) > 9 && frame[0] == roomFrameUpdate {
			break
		}
	}
	room, err := s.GetOrCreateLiveRoom(ctx, session.ID, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendLiveRoomUpdate(ctx, room.ID, []byte("missed-update")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FreezeLiveRoom(ctx, room.ID, "teacher_forced"); err != nil {
		t.Fatal(err)
	}
	h.roomHub.Close(room.ID)
	receivedFinal := false
	for {
		_, frame, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, 4000) {
				t.Fatal(err)
			}
			break
		}
		if len(frame) > 9 && string(frame[9:]) == "missed-update" {
			receivedFinal = true
		}
	}
	if !receivedFinal {
		t.Fatal("freeze discarded committed update before final replay")
	}
	_ = conn.Close()
	select {
	case <-roomWSFinished:
	case <-time.After(time.Second):
		t.Fatal("RoomWS did not finish after client close")
	}

	room, err = s.GetOrCreateLiveRoom(ctx, session.ID, item.ID)
	if err != nil {
		t.Fatalf("GetOrCreateLiveRoom: %v", err)
	}
	updates, err := s.ReadLiveRoomUpdates(ctx, room.ID, 0)
	if err != nil {
		t.Fatalf("GetLiveRoomDocSnapshot: %v", err)
	}
	if len(updates) != 2 || string(updates[0].Payload) != "teacher-yjs-state" {
		t.Errorf("journal = %v, want teacher websocket update", updates)
	}
	var participantCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM live_room_participants WHERE room_id = $1 AND user_id = $2`, room.ID, teacher.ID,
	).Scan(&participantCount); err != nil {
		t.Fatalf("count teacher presence rows: %v", err)
	}
	if participantCount != 0 {
		t.Errorf("teacher presence rows = %d, want 0", participantCount)
	}
}

// TestForceEndRoom_TeacherOnlyThenFreezesAndJudges exercises the collaborative
// room lifecycle end to end at the HTTP layer: a student cannot force-end a
// room, and a teacher's force-end freezes it and — since the item has a
// linked challenge — enqueues a judge submission from the room's last known
// plain-text snapshot, with no client ever opening the websocket relay.
func TestForceEndRoom_TeacherOnlyThenFreezesAndJudges(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	teacher, err := s.CreateUser(ctx, "live-room-force-end-teacher@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser teacher: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID) })

	student, err := s.CreateUser(ctx, "live-room-force-end-student@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser student: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, student.ID) })

	lang := "python"
	challenge, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "live-room-force-end-challenge", Title: "Force End Challenge", Difficulty: "easy", Language: &lang,
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Force End List"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteProblemList(ctx, list.ID, teacher.ID) })

	item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
		Title: "Item", Difficulty: "easy", LinkedChallengeID: &challenge.ID,
	})
	if err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}

	session, err := s.CreateLiveSessionWithMode(ctx, list.ID, teacher.ID, 1, "collaborative")
	if err != nil {
		t.Fatalf("CreateLiveSessionWithMode: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM live_sessions WHERE id = $1`, session.ID) })
	// Registered last so it runs first (t.Cleanup is LIFO): submissions
	// reference both live_room_id and live_session_id, so they must clear
	// before the live_sessions delete above can succeed.
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, challenge.ID)
		_ = s.DeleteChallenge(ctx, challenge.ID)
	})

	room, err := s.GetOrCreateLiveRoom(ctx, session.ID, item.ID)
	if err != nil {
		t.Fatalf("GetOrCreateLiveRoom: %v", err)
	}
	if _, err := s.JoinLiveRoom(ctx, room.ID, student.ID); err != nil {
		t.Fatalf("JoinLiveRoom: %v", err)
	}
	if err := s.SaveLiveRoomTextSnapshot(ctx, room.ID, "print('final answer')"); err != nil {
		t.Fatalf("SaveLiveRoomTextSnapshot: %v", err)
	}

	h := NewLiveSessionsHandler(s, nil, nil)
	r := chi.NewRouter()
	r.Post("/live-sessions/{id}/rooms/{list_item_id}/force-end", h.ForceEndRoom)

	asStudent := auth.Claims{UserID: student.ID, Email: student.Email, Role: "student", RealRole: "student"}
	asTeacher := auth.Claims{UserID: teacher.ID, Email: teacher.Email, Role: "teacher", RealRole: "teacher"}

	path := "/live-sessions/" + session.ID + "/rooms/" + item.ID + "/force-end"

	studentReq := httptest.NewRequest(http.MethodPost, path, nil)
	studentReq = studentReq.WithContext(auth.NewContext(studentReq.Context(), asStudent))
	studentW := httptest.NewRecorder()
	r.ServeHTTP(studentW, studentReq)
	if studentW.Code != http.StatusForbidden {
		t.Fatalf("student force-end: got status %d, want 403", studentW.Code)
	}

	teacherReq := httptest.NewRequest(http.MethodPost, path, nil)
	teacherReq = teacherReq.WithContext(auth.NewContext(teacherReq.Context(), asTeacher))
	teacherW := httptest.NewRecorder()
	r.ServeHTTP(teacherW, teacherReq)
	if teacherW.Code != http.StatusNoContent {
		t.Fatalf("teacher force-end: got status %d, want 204", teacherW.Code)
	}

	frozen, err := s.GetLiveRoom(ctx, room.ID)
	if err != nil {
		t.Fatalf("GetLiveRoom: %v", err)
	}
	if frozen.Status != "frozen" || frozen.FrozenReason == nil || *frozen.FrozenReason != "teacher_forced" {
		t.Fatalf("unexpected room state after force-end: %+v", frozen)
	}

	sub, err := s.LiveRoomResultSubmission(ctx, room.ID)
	if err != nil {
		t.Fatalf("LiveRoomResultSubmission: %v", err)
	}
	if sub.SourceCode != "print('final answer')" {
		t.Errorf("submission source: got %q, want the room's last snapshot", sub.SourceCode)
	}
	if sub.Language != "python" {
		t.Errorf("submission language: got %q, want python", sub.Language)
	}

	// A second force-end on an already-frozen room must not error or spawn
	// a duplicate submission.
	teacherReq2 := httptest.NewRequest(http.MethodPost, path, nil)
	teacherReq2 = teacherReq2.WithContext(auth.NewContext(teacherReq2.Context(), asTeacher))
	teacherW2 := httptest.NewRecorder()
	r.ServeHTTP(teacherW2, teacherReq2)
	if teacherW2.Code != http.StatusNoContent {
		t.Fatalf("repeat force-end: got status %d, want 204", teacherW2.Code)
	}
}
