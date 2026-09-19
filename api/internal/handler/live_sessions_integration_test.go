//go:build integration

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestCreateLiveSession_ListWithoutLinkedChallenge_Returns422 locks in the
// existing backend guard this feature depends on: a Live Session can't be
// backed by a problem list whose items are all self-graded (no
// linked_challenge_id), because live-session progress is tracked entirely
// from judge submissions against linked challenges. The frontend now
// disables such lists in the creation UI (see LiveSessionsPage.tsx /
// ProblemList.live_session_eligible), but this 422 must keep working
// independently of that, since it's the actual source of truth.
func TestCreateLiveSession_ListWithoutLinkedChallenge_Returns422(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	teacher, err := s.CreateUser(ctx, "live-create-no-link-teacher@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, teacher.ID) })

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Self-graded only"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteProblemList(ctx, list.ID, teacher.ID) })
	if _, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
		Title: "Exercise", Difficulty: "easy", Body: "no judge integration",
	}); err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}

	h := NewLiveSessionsHandler(s)
	r := chi.NewRouter()
	r.Post("/live-sessions", h.Create)

	body := `{"problem_list_id":"` + list.ID + `","min_participants":1}`
	req := httptest.NewRequest(http.MethodPost, "/live-sessions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.NewContext(req.Context(), auth.Claims{
		UserID: teacher.ID, Email: teacher.Email, Role: "teacher", RealRole: "teacher",
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("needs at least one linked challenge")) {
		t.Errorf("expected error message about linked challenge requirement, got: %s", w.Body.String())
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

func TestTimedLiveSessionRounds_StartEndAndStatus(t *testing.T) {
	db := openHandlerTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher, err := s.CreateUser(ctx, "timed-round-teacher@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser teacher: %v", err)
	}
	student, err := s.CreateUser(ctx, "timed-round-student@example.com", "unused-hash")
	if err != nil {
		t.Fatalf("CreateUser student: %v", err)
	}
	challenge, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{Slug: "timed-round-challenge", Title: "Timed Round", Difficulty: "easy"})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Timed list"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	if _, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{Title: "Round one", Difficulty: "easy", LinkedChallengeID: &challenge.ID}); err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}
	session, err := s.CreateLiveSessionWithMode(ctx, list.ID, teacher.ID, 1, "timed_challenge")
	if err != nil {
		t.Fatalf("CreateLiveSessionWithMode: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM live_sessions WHERE id=$1`, session.ID)
		_ = s.DeleteProblemList(ctx, list.ID, teacher.ID)
		_ = s.DeleteChallenge(ctx, challenge.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, teacher.ID, student.ID)
	})
	if _, err := s.JoinLiveSession(ctx, session.ID, student.ID); err != nil {
		t.Fatalf("JoinLiveSession: %v", err)
	}
	rounds, err := s.CreateLiveSessionRounds(ctx, session.ID, teacher.ID)
	if err != nil || len(rounds) != 1 {
		t.Fatalf("CreateLiveSessionRounds = %#v, %v", rounds, err)
	}
	active, err := s.StartLiveSessionRound(ctx, session.ID, rounds[0].ID, teacher.ID, 60)
	if err != nil || active.Status != "active" || active.StartedAt == nil {
		t.Fatalf("StartLiveSessionRound = %#v, %v", active, err)
	}
	if err := s.ValidateLiveSubmission(ctx, session.ID, student.ID, challenge.ID); err != nil {
		t.Fatalf("ValidateLiveSubmission active round: %v", err)
	}
	round, statuses, err := s.LiveRoundStatus(ctx, session.ID, active.ID, teacher.ID)
	if err != nil || round.ID != active.ID || len(statuses) != 1 || statuses[0].Status != "not_started" {
		t.Fatalf("LiveRoundStatus = %#v %#v %v", round, statuses, err)
	}
	ended, err := s.EndLiveSessionRound(ctx, session.ID, active.ID, teacher.ID)
	if err != nil || ended.Status != "ended" || ended.EndedAt == nil {
		t.Fatalf("EndLiveSessionRound = %#v, %v", ended, err)
	}
	if err := s.ValidateLiveSubmission(ctx, session.ID, student.ID, challenge.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("ValidateLiveSubmission after end = %v, want ErrNotFound", err)
	}
}
