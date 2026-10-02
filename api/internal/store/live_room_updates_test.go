//go:build integration

package store_test

import (
	"context"
	"sync"
	"testing"

	"github.com/caze/ascend/api/internal/store"
)

func TestLiveRoomJournalAndOutOfOrderSnapshots(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	session, item := setupCollaborativeRoom(t, s, db, ctx)
	room, err := s.GetOrCreateLiveRoom(ctx, session, item)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveLiveRoomDocSnapshot(ctx, room.ID, []byte("legacy-baseline")); err != nil {
		t.Fatal(err)
	}
	first, err := s.AppendLiveRoomUpdate(ctx, room.ID, []byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.AppendLiveRoomUpdate(ctx, room.ID, []byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	if saved, err := s.SaveLiveRoomSnapshot(ctx, room.ID, second, []byte("new"), "new"); err != nil || !saved {
		t.Fatalf("new snapshot: %v %v", saved, err)
	}
	if saved, err := s.SaveLiveRoomSnapshot(ctx, room.ID, first, []byte("old"), "old"); err != nil || saved {
		t.Fatalf("stale snapshot: %v %v", saved, err)
	}
	doc, err := s.GetLiveRoomDocSnapshot(ctx, room.ID)
	if err != nil || string(doc) != "new" {
		t.Fatalf("cache rolled back: %q %v", doc, err)
	}
	if err := s.SaveLiveRoomDocSnapshot(ctx, room.ID, []byte("legacy-late")); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveLiveRoomTextSnapshot(ctx, room.ID, "legacy-late"); err != nil {
		t.Fatal(err)
	}
	current, err := s.GetLiveRoom(ctx, room.ID)
	if err != nil || current.TextSnapshot != "new" {
		t.Fatalf("text cache rolled back: %+v %v", current, err)
	}
	updates, err := s.ReadLiveRoomUpdates(ctx, room.ID, -1)
	if err != nil || len(updates) != 3 || string(updates[0].Payload) != "legacy-baseline" || string(updates[1].Payload) != "first" || string(updates[2].Payload) != "second" {
		t.Fatalf("incomplete journal: %+v %v", updates, err)
	}
	// Racing writers must commit a gap-free sequence, even across connections.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.AppendLiveRoomUpdate(ctx, room.ID, []byte("concurrent")); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	updates, err = s.ReadLiveRoomUpdates(ctx, room.ID, 0)
	if err != nil || len(updates) != 22 {
		t.Fatalf("journal count: %d %v", len(updates), err)
	}
	for i, u := range updates {
		if u.Version != uint64(i+1) {
			t.Fatalf("gap at %d: %d", i, u.Version)
		}
	}
	if _, err := s.FreezeLiveRoom(ctx, room.ID, "teacher_forced"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendLiveRoomUpdate(ctx, room.ID, []byte("frozen")); err != store.ErrConflict {
		t.Fatalf("frozen update: %v", err)
	}
}
