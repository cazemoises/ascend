//go:build integration

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

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
