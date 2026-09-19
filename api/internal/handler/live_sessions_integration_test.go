//go:build integration

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

// TestLiveSessionPreview_DoesNotExposeClassroomDataBeforeJoin protects the
// join flow without making a live classroom's roster or challenge list public
// to every authenticated account. A student needs the session metadata to
// decide to join; the sensitive content belongs only to participants and the
// session's teacher.
func TestLiveSessionPreview_DoesNotExposeClassroomDataBeforeJoin(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	teacher, err := s.CreateUser(ctx, "live-preview-teacher@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser teacher: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID) })

	student, err := s.CreateUser(ctx, "live-preview-student@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser student: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, student.ID) })

	challenge, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "live-preview-challenge", Title: "Live Preview Challenge", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, challenge.ID)
		_ = s.DeleteChallenge(ctx, challenge.ID)
	})

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Live Preview List"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteProblemList(ctx, list.ID, teacher.ID) })
	if _, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
		Title: "Challenge", Difficulty: "easy", LinkedChallengeID: &challenge.ID,
	}); err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}

	session, err := s.CreateLiveSession(ctx, list.ID, teacher.ID, 2)
	if err != nil {
		t.Fatalf("CreateLiveSession: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM live_sessions WHERE id = $1`, session.ID) })

	h := NewLiveSessionsHandler(s)
	r := chi.NewRouter()
	r.Get("/live-sessions/{id}", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/live-sessions/"+session.ID, nil)
	req = req.WithContext(auth.NewContext(req.Context(), auth.Claims{
		UserID: student.ID, Email: student.Email, Role: "student", RealRole: "student",
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET live session preview: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got struct {
		Session struct {
			Joined bool             `json:"joined"`
			Items  []store.ListItem `json:"items"`
		} `json:"session"`
		Participants []store.LiveParticipant `json:"participants"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Session.Joined {
		t.Fatal("previewing student unexpectedly appears joined")
	}
	if len(got.Session.Items) != 0 {
		t.Errorf("preview exposed %d challenge(s), want none before joining", len(got.Session.Items))
	}
	if len(got.Participants) != 0 {
		t.Errorf("preview exposed %d participant(s), want no roster before joining", len(got.Participants))
	}
}

// TestLiveSessionJoin_RejectsTeacherPreview keeps View As Student a UI
// preview rather than a way for a teacher identity to become a participant
// and later add teacher-owned submissions to classroom progress.
func TestLiveSessionJoin_RejectsTeacherPreview(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	teacher, err := s.CreateUser(ctx, "live-join-teacher@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID) })

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Live Join List"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteProblemList(ctx, list.ID, teacher.ID) })

	challenge, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "live-join-challenge", Title: "Live Join Challenge", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteChallenge(ctx, challenge.ID) })
	if _, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
		Title: "Challenge", Difficulty: "easy", LinkedChallengeID: &challenge.ID,
	}); err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}

	session, err := s.CreateLiveSession(ctx, list.ID, teacher.ID, 1)
	if err != nil {
		t.Fatalf("CreateLiveSession: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM live_sessions WHERE id = $1`, session.ID) })

	h := NewLiveSessionsHandler(s)
	r := chi.NewRouter()
	r.Post("/live-sessions/{id}/join", h.Join)
	req := httptest.NewRequest(http.MethodPost, "/live-sessions/"+session.ID+"/join", nil)
	req = req.WithContext(auth.NewContext(req.Context(), auth.Claims{
		UserID: teacher.ID, Email: teacher.Email, Role: "student", RealRole: "teacher",
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("teacher preview POST join: expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
