//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/caze/ascend/api/internal/store"
)

func TestCreateChallengeGroup_OrdinalDefaultsToNextAvailable(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "groups-ordinal-teacher@example.com")

	first, err := s.CreateChallengeGroup(ctx, store.CreateChallengeGroupRequest{
		TeacherID: teacher.ID, Title: "First",
	})
	if err != nil {
		t.Fatalf("CreateChallengeGroup (first): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeGroup(ctx, first.ID, teacher.ID) })
	if first.Ordinal != 0 {
		t.Errorf("first ordinal = %d, want 0", first.Ordinal)
	}

	second, err := s.CreateChallengeGroup(ctx, store.CreateChallengeGroupRequest{
		TeacherID: teacher.ID, Title: "Second",
	})
	if err != nil {
		t.Fatalf("CreateChallengeGroup (second): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeGroup(ctx, second.ID, teacher.ID) })
	if second.Ordinal != 1 {
		t.Errorf("second ordinal = %d, want 1", second.Ordinal)
	}
}

func TestUpdateChallengeGroup_NotOwnerNotFound(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	owner := createTestUser(t, s, db, ctx, "groups-owner@example.com")
	other := createTestUser(t, s, db, ctx, "groups-other@example.com")

	cg, err := s.CreateChallengeGroup(ctx, store.CreateChallengeGroupRequest{
		TeacherID: owner.ID, Title: "Owner's",
	})
	if err != nil {
		t.Fatalf("CreateChallengeGroup: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeGroup(ctx, cg.ID, owner.ID) })

	_, err = s.UpdateChallengeGroup(ctx, cg.ID, other.ID, store.UpdateChallengeGroupRequest{
		Title: "Hijacked", Ordinal: 0,
	})
	if err != store.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound (non-owner can't update)", err)
	}

	if err := s.DeleteChallengeGroup(ctx, cg.ID, other.ID); err != store.ErrNotFound {
		t.Errorf("delete err = %v, want ErrNotFound (non-owner can't delete)", err)
	}
}

func TestDeleteChallengeGroup_UnlinksCollectionsInsteadOfBlocking(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "groups-delete-teacher@example.com")

	cg, err := s.CreateChallengeGroup(ctx, store.CreateChallengeGroupRequest{
		TeacherID: teacher.ID, Title: "To Delete",
	})
	if err != nil {
		t.Fatalf("CreateChallengeGroup: %v", err)
	}

	cc, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: teacher.ID, Title: "In Group", GroupID: &cg.ID,
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, cc.ID, teacher.ID) })

	if err := s.DeleteChallengeGroup(ctx, cg.ID, teacher.ID); err != nil {
		t.Fatalf("DeleteChallengeGroup: %v", err)
	}

	reloaded, err := s.ListChallengeCollections(ctx, teacher.ID)
	if err != nil {
		t.Fatalf("ListChallengeCollections: %v", err)
	}
	var found bool
	for _, c := range reloaded {
		if c.ID == cc.ID {
			found = true
			if c.GroupID != nil {
				t.Errorf("GroupID = %v, want nil (ON DELETE SET NULL)", c.GroupID)
			}
		}
	}
	if !found {
		t.Fatal("collection must survive group deletion")
	}
}

func TestCreateChallengeCollection_GroupNotOwnedRejected(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	owner := createTestUser(t, s, db, ctx, "groups-collection-owner@example.com")
	other := createTestUser(t, s, db, ctx, "groups-collection-other@example.com")

	cg, err := s.CreateChallengeGroup(ctx, store.CreateChallengeGroupRequest{
		TeacherID: owner.ID, Title: "Owner's Group",
	})
	if err != nil {
		t.Fatalf("CreateChallengeGroup: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeGroup(ctx, cg.ID, owner.ID) })

	_, err = s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: other.ID, Title: "Trying to nest under someone else's group", GroupID: &cg.ID,
	})
	if err != store.ErrGroupNotFound {
		t.Errorf("err = %v, want ErrGroupNotFound (group belongs to a different teacher)", err)
	}

	bogusID := "00000000-0000-0000-0000-000000000000"
	_, err = s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: owner.ID, Title: "Bogus group", GroupID: &bogusID,
	})
	if err != store.ErrGroupNotFound {
		t.Errorf("err = %v, want ErrGroupNotFound (group doesn't exist)", err)
	}
}

func TestReorderChallengeGroups_AtomicOnPartialFailure(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "groups-reorder-atomic-teacher@example.com")

	cg, err := s.CreateChallengeGroup(ctx, store.CreateChallengeGroupRequest{
		TeacherID: teacher.ID, Title: "Solo",
	})
	if err != nil {
		t.Fatalf("CreateChallengeGroup: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeGroup(ctx, cg.ID, teacher.ID) })

	err = s.ReorderChallengeGroups(ctx, teacher.ID, []store.GroupReorderItem{
		{ID: cg.ID, Ordinal: 9},
		{ID: "00000000-0000-0000-0000-000000000000", Ordinal: 10},
	})
	if err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound from the batch, got %v", err)
	}

	cgs, err := s.ListChallengeGroups(ctx, teacher.ID)
	if err != nil {
		t.Fatalf("ListChallengeGroups: %v", err)
	}
	if cgs[0].Ordinal != cg.Ordinal {
		t.Errorf("ordinal = %d, want %d (unchanged — the batch should have rolled back)", cgs[0].Ordinal, cg.Ordinal)
	}
}

// TestListChallengesForViewer_GroupedByGroupThenCollection covers the
// two-level ordering: group.ordinal, then collection.ordinal within the
// group, then created_at within the collection — with an ungrouped
// collection's challenges sorting after every grouped one.
func TestListChallengesForViewer_GroupedByGroupThenCollection(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "groups-ordering-teacher@example.com")

	group, err := s.CreateChallengeGroup(ctx, store.CreateChallengeGroupRequest{
		TeacherID: teacher.ID, Title: "SQL",
	})
	if err != nil {
		t.Fatalf("CreateChallengeGroup: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeGroup(ctx, group.ID, teacher.ID) })

	ccInGroup, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: teacher.ID, Title: "SQL Iniciante", GroupID: &group.ID,
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection (in group): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, ccInGroup.ID, teacher.ID) })

	ccUngrouped, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: teacher.ID, Title: "Ungrouped Collection",
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection (ungrouped): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, ccUngrouped.ID, teacher.ID) })

	chGrouped, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "groups-ordering-grouped", Title: "In grouped collection", Difficulty: "easy",
		CollectionID: &ccInGroup.ID,
	})
	if err != nil {
		t.Fatalf("CreateChallenge (grouped): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, chGrouped.ID) })

	chUngrouped, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "groups-ordering-ungrouped", Title: "In ungrouped collection", Difficulty: "easy",
		CollectionID: &ccUngrouped.ID,
	})
	if err != nil {
		t.Fatalf("CreateChallenge (ungrouped): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, chUngrouped.ID) })

	items, err := s.ListChallengesForViewer(ctx, "", 100, 0)
	if err != nil {
		t.Fatalf("ListChallengesForViewer: %v", err)
	}

	indexOf := func(id string) int {
		for i, it := range items {
			if it.ID == id {
				return i
			}
		}
		t.Fatalf("challenge %s not found in feed", id)
		return -1
	}

	idxGrouped := indexOf(chGrouped.ID)
	idxUngrouped := indexOf(chUngrouped.ID)
	if idxGrouped >= idxUngrouped {
		t.Errorf("grouped collection's challenge must sort before an ungrouped collection's: idxGrouped=%d idxUngrouped=%d", idxGrouped, idxUngrouped)
	}

	for _, it := range items {
		switch it.ID {
		case chGrouped.ID:
			if it.GroupID == nil || *it.GroupID != group.ID {
				t.Errorf("chGrouped GroupID = %v, want %q", it.GroupID, group.ID)
			}
			if it.GroupTitle == nil || *it.GroupTitle != "SQL" {
				t.Errorf("chGrouped GroupTitle = %v, want %q", it.GroupTitle, "SQL")
			}
		case chUngrouped.ID:
			if it.GroupID != nil {
				t.Errorf("chUngrouped GroupID = %v, want nil", it.GroupID)
			}
		}
	}
}
