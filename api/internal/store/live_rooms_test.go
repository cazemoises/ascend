//go:build integration

package store_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/caze/ascend/api/internal/store"
)

// setupCollaborativeRoom creates a teacher, a collaborative-mode session
// with one linked-challenge list item, and returns everything a room test
// needs. Cleanups are registered in dependency order (session, then list,
// then challenge; teacher is deleted last by createTestUser's own cleanup)
// so each FK actually clears instead of silently failing and leaking rows
// into the next test — the live_sessions row must go first, since
// problem_lists/users deletes are blocked while it still references them.
func setupCollaborativeRoom(t *testing.T, s *store.Store, db *sql.DB, ctx context.Context) (sessionID, listItemID string) {
	t.Helper()
	slug := strings.ToLower(strings.NewReplacer("/", "-", " ", "-").Replace(t.Name()))
	teacher := createTestUser(t, s, db, ctx, "live-room-teacher-"+slug+"@example.com")

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "live-room-challenge-" + slug, Title: "Live Room Challenge", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteChallenge(ctx, ch.ID) })

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Live Room List"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteProblemList(ctx, list.ID, teacher.ID) })

	item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
		Title: "Item", Difficulty: "easy", LinkedChallengeID: &ch.ID,
	})
	if err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}

	session, err := s.CreateLiveSessionWithMode(ctx, list.ID, teacher.ID, 1, "collaborative")
	if err != nil {
		t.Fatalf("CreateLiveSessionWithMode: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM live_sessions WHERE id = $1`, session.ID) })
	return session.ID, item.ID
}

// dropParticipantRowsOnCleanup ensures a user's live_room_participants rows
// are gone before createTestUser's own (earlier-registered) cleanup tries to
// delete the user — t.Cleanup runs LIFO, so this must be registered after
// createTestUser to run first, rather than racing the live_sessions cascade.
func dropParticipantRowsOnCleanup(t *testing.T, db *sql.DB, ctx context.Context, userID string) {
	t.Helper()
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM live_room_participants WHERE user_id = $1`, userID) })
}

func TestGetOrCreateLiveRoom_BornOnDemand(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	sessionID, itemID := setupCollaborativeRoom(t, s, db, ctx)

	first, err := s.GetOrCreateLiveRoom(ctx, sessionID, itemID)
	if err != nil {
		t.Fatalf("GetOrCreateLiveRoom: %v", err)
	}
	if first.Status != "open" {
		t.Errorf("status: got %q, want open", first.Status)
	}

	second, err := s.GetOrCreateLiveRoom(ctx, sessionID, itemID)
	if err != nil {
		t.Fatalf("GetOrCreateLiveRoom (again): %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("expected the same room on repeat access, got %q then %q", first.ID, second.ID)
	}

	rooms, err := s.ListLiveRoomsForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("ListLiveRoomsForSession: %v", err)
	}
	if len(rooms) != 1 || rooms[0].RoomID == nil || *rooms[0].RoomID != first.ID {
		t.Fatalf("expected one summary row pointing at the created room, got %+v", rooms)
	}
}

func TestJoinLeaveLiveRoom_PresenceCount(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	sessionID, itemID := setupCollaborativeRoom(t, s, db, ctx)
	room, err := s.GetOrCreateLiveRoom(ctx, sessionID, itemID)
	if err != nil {
		t.Fatalf("GetOrCreateLiveRoom: %v", err)
	}

	u1 := createTestUser(t, s, db, ctx, "live-room-student-1@example.com")
	dropParticipantRowsOnCleanup(t, db, ctx, u1.ID)
	u2 := createTestUser(t, s, db, ctx, "live-room-student-2@example.com")
	dropParticipantRowsOnCleanup(t, db, ctx, u2.ID)

	p1, err := s.JoinLiveRoom(ctx, room.ID, u1.ID)
	if err != nil {
		t.Fatalf("JoinLiveRoom u1: %v", err)
	}
	if _, err := s.JoinLiveRoom(ctx, room.ID, u2.ID); err != nil {
		t.Fatalf("JoinLiveRoom u2: %v", err)
	}

	rooms, err := s.ListLiveRoomsForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("ListLiveRoomsForSession: %v", err)
	}
	if rooms[0].PresentCount != 2 {
		t.Fatalf("present count: got %d, want 2", rooms[0].PresentCount)
	}

	if err := s.LeaveLiveRoom(ctx, p1); err != nil {
		t.Fatalf("LeaveLiveRoom: %v", err)
	}
	rooms, err = s.ListLiveRoomsForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("ListLiveRoomsForSession (after leave): %v", err)
	}
	if rooms[0].PresentCount != 1 {
		t.Fatalf("present count after leave: got %d, want 1", rooms[0].PresentCount)
	}
}

func TestMarkLiveRoomDone_RequiresEveryPresentParticipant(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	sessionID, itemID := setupCollaborativeRoom(t, s, db, ctx)
	room, err := s.GetOrCreateLiveRoom(ctx, sessionID, itemID)
	if err != nil {
		t.Fatalf("GetOrCreateLiveRoom: %v", err)
	}

	u1 := createTestUser(t, s, db, ctx, "live-room-done-1@example.com")
	dropParticipantRowsOnCleanup(t, db, ctx, u1.ID)
	u2 := createTestUser(t, s, db, ctx, "live-room-done-2@example.com")
	dropParticipantRowsOnCleanup(t, db, ctx, u2.ID)
	if _, err := s.JoinLiveRoom(ctx, room.ID, u1.ID); err != nil {
		t.Fatalf("JoinLiveRoom u1: %v", err)
	}
	if _, err := s.JoinLiveRoom(ctx, room.ID, u2.ID); err != nil {
		t.Fatalf("JoinLiveRoom u2: %v", err)
	}

	allDone, err := s.MarkLiveRoomDone(ctx, room.ID, u1.ID)
	if err != nil {
		t.Fatalf("MarkLiveRoomDone u1: %v", err)
	}
	if allDone {
		t.Fatal("expected allDone=false with u2 not yet marked")
	}

	allDone, err = s.MarkLiveRoomDone(ctx, room.ID, u2.ID)
	if err != nil {
		t.Fatalf("MarkLiveRoomDone u2: %v", err)
	}
	if !allDone {
		t.Fatal("expected allDone=true once every present participant marked done")
	}
}

func TestMarkLiveRoomDone_ReopeningResetsTheCheck(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	sessionID, itemID := setupCollaborativeRoom(t, s, db, ctx)
	room, err := s.GetOrCreateLiveRoom(ctx, sessionID, itemID)
	if err != nil {
		t.Fatalf("GetOrCreateLiveRoom: %v", err)
	}

	u1 := createTestUser(t, s, db, ctx, "live-room-reopen-1@example.com")
	dropParticipantRowsOnCleanup(t, db, ctx, u1.ID)
	u2 := createTestUser(t, s, db, ctx, "live-room-reopen-2@example.com")
	dropParticipantRowsOnCleanup(t, db, ctx, u2.ID)
	if _, err := s.JoinLiveRoom(ctx, room.ID, u1.ID); err != nil {
		t.Fatalf("JoinLiveRoom u1: %v", err)
	}
	allDone, err := s.MarkLiveRoomDone(ctx, room.ID, u1.ID)
	if err != nil {
		t.Fatalf("MarkLiveRoomDone u1: %v", err)
	}
	if !allDone {
		t.Fatal("expected allDone=true with only u1 present and marked")
	}

	// u2 joins after u1 already marked done — the room must not be
	// considered all-done again until u2 also marks done.
	if _, err := s.JoinLiveRoom(ctx, room.ID, u2.ID); err != nil {
		t.Fatalf("JoinLiveRoom u2: %v", err)
	}
	rooms, err := s.ListLiveRoomsForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("ListLiveRoomsForSession: %v", err)
	}
	if rooms[0].PresentCount != 2 {
		t.Fatalf("present count: got %d, want 2", rooms[0].PresentCount)
	}
}

func TestFreezeLiveRoom_IdempotentAcrossTriggers(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	sessionID, itemID := setupCollaborativeRoom(t, s, db, ctx)
	room, err := s.GetOrCreateLiveRoom(ctx, sessionID, itemID)
	if err != nil {
		t.Fatalf("GetOrCreateLiveRoom: %v", err)
	}

	frozen, err := s.FreezeLiveRoom(ctx, room.ID, "all_done")
	if err != nil {
		t.Fatalf("FreezeLiveRoom: %v", err)
	}
	if frozen.Status != "frozen" || frozen.FrozenReason == nil || *frozen.FrozenReason != "all_done" {
		t.Fatalf("unexpected frozen room: %+v", frozen)
	}

	// A second trigger (e.g. teacher force-end racing the all-done check)
	// must not error loudly or overwrite the original reason.
	if _, err := s.FreezeLiveRoom(ctx, room.ID, "teacher_forced"); err != store.ErrConflict {
		t.Fatalf("expected ErrConflict on double-freeze, got %v", err)
	}
	again, err := s.GetLiveRoom(ctx, room.ID)
	if err != nil {
		t.Fatalf("GetLiveRoom: %v", err)
	}
	if *again.FrozenReason != "all_done" {
		t.Fatalf("frozen_reason changed on a no-op refreeze: got %q, want all_done", *again.FrozenReason)
	}
}

func TestFreezeOpenLiveRoomsForSession_OnlyOpenRooms(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	sessionID, itemID := setupCollaborativeRoom(t, s, db, ctx)
	room, err := s.GetOrCreateLiveRoom(ctx, sessionID, itemID)
	if err != nil {
		t.Fatalf("GetOrCreateLiveRoom: %v", err)
	}

	frozen, err := s.FreezeOpenLiveRoomsForSession(ctx, sessionID, "time_limit")
	if err != nil {
		t.Fatalf("FreezeOpenLiveRoomsForSession: %v", err)
	}
	if len(frozen) != 1 || frozen[0].ID != room.ID || *frozen[0].FrozenReason != "time_limit" {
		t.Fatalf("unexpected freeze-all result: %+v", frozen)
	}

	// Already-frozen rooms are excluded on a repeat call (session finished twice).
	again, err := s.FreezeOpenLiveRoomsForSession(ctx, sessionID, "time_limit")
	if err != nil {
		t.Fatalf("FreezeOpenLiveRoomsForSession (again): %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("expected no rooms left to freeze, got %d", len(again))
	}
}

func TestValidateLiveRoomItem_RejectsWrongSessionOrMode(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	sessionID, itemID := setupCollaborativeRoom(t, s, db, ctx)

	if err := s.ValidateLiveRoomItem(ctx, sessionID, itemID); err != nil {
		t.Fatalf("expected the real session/item pair to validate, got %v", err)
	}
	if err := s.ValidateLiveRoomItem(ctx, sessionID, "00000000-0000-0000-0000-000000000000"); err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound for an unrelated item, got %v", err)
	}
}
