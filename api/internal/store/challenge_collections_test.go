//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/caze/ascend/api/internal/store"
)

func TestCreateChallengeCollection_OrdinalDefaultsToNextAvailable(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "collections-ordinal-teacher@example.com")

	first, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: teacher.ID, Title: "First",
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection (first): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, first.ID, teacher.ID) })
	if first.Ordinal != 0 {
		t.Errorf("first ordinal = %d, want 0", first.Ordinal)
	}

	second, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: teacher.ID, Title: "Second",
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection (second): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, second.ID, teacher.ID) })
	if second.Ordinal != 1 {
		t.Errorf("second ordinal = %d, want 1", second.Ordinal)
	}

	explicit := 5
	third, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: teacher.ID, Title: "Third", Ordinal: &explicit,
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection (third): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, third.ID, teacher.ID) })
	if third.Ordinal != 5 {
		t.Errorf("third ordinal = %d, want 5 (explicit value honored)", third.Ordinal)
	}
}

func TestUpdateChallengeCollection_NotOwnerNotFound(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	owner := createTestUser(t, s, db, ctx, "collections-owner@example.com")
	other := createTestUser(t, s, db, ctx, "collections-other@example.com")

	cc, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: owner.ID, Title: "Owner's",
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, cc.ID, owner.ID) })

	_, err = s.UpdateChallengeCollection(ctx, cc.ID, other.ID, store.UpdateChallengeCollectionRequest{
		Title: "Hijacked", Ordinal: 0,
	})
	if err != store.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound (non-owner can't update)", err)
	}

	if err := s.DeleteChallengeCollection(ctx, cc.ID, other.ID); err != store.ErrNotFound {
		t.Errorf("delete err = %v, want ErrNotFound (non-owner can't delete)", err)
	}
}

func TestDeleteChallengeCollection_UnlinksChallengesInsteadOfBlocking(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "collections-delete-teacher@example.com")

	cc, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: teacher.ID, Title: "To Delete",
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection: %v", err)
	}

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "collections-delete-challenge", Title: "In Collection", Difficulty: "easy",
		CollectionID: &cc.ID,
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	if err := s.DeleteChallengeCollection(ctx, cc.ID, teacher.ID); err != nil {
		t.Fatalf("DeleteChallengeCollection: %v", err)
	}

	reloaded, err := s.GetChallenge(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetChallenge: %v (challenge must survive collection deletion)", err)
	}
	if reloaded.CollectionID != nil {
		t.Errorf("CollectionID = %v, want nil (ON DELETE SET NULL)", reloaded.CollectionID)
	}
}

func TestCreateChallenge_CollectionNotFoundRejected(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	bogusID := "00000000-0000-0000-0000-000000000000"
	_, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "collections-bogus-id-challenge", Title: "Bogus", Difficulty: "easy",
		CollectionID: &bogusID,
	})
	if err != store.ErrCollectionNotFound {
		t.Errorf("err = %v, want ErrCollectionNotFound", err)
	}
}

func TestReorderChallengeCollections_HappyPathSwapsOrdinals(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "collections-reorder-teacher@example.com")

	first, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: teacher.ID, Title: "First",
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection (first): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, first.ID, teacher.ID) })

	second, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: teacher.ID, Title: "Second",
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection (second): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, second.ID, teacher.ID) })

	err = s.ReorderChallengeCollections(ctx, teacher.ID, []store.CollectionReorderItem{
		{ID: first.ID, Ordinal: second.Ordinal},
		{ID: second.ID, Ordinal: first.Ordinal},
	})
	if err != nil {
		t.Fatalf("ReorderChallengeCollections: %v", err)
	}

	ccs, err := s.ListChallengeCollections(ctx, teacher.ID)
	if err != nil {
		t.Fatalf("ListChallengeCollections: %v", err)
	}
	for _, cc := range ccs {
		switch cc.ID {
		case first.ID:
			if cc.Ordinal != second.Ordinal {
				t.Errorf("first ordinal = %d, want %d (swapped)", cc.Ordinal, second.Ordinal)
			}
		case second.ID:
			if cc.Ordinal != first.Ordinal {
				t.Errorf("second ordinal = %d, want %d (swapped)", cc.Ordinal, first.Ordinal)
			}
		}
	}
}

func TestReorderChallengeCollections_AtomicOnPartialFailure(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "collections-reorder-atomic-teacher@example.com")

	cc, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: teacher.ID, Title: "Solo",
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, cc.ID, teacher.ID) })

	// The second entry targets a nonexistent collection id — the whole batch
	// must roll back, leaving cc's ordinal untouched.
	err = s.ReorderChallengeCollections(ctx, teacher.ID, []store.CollectionReorderItem{
		{ID: cc.ID, Ordinal: 9},
		{ID: "00000000-0000-0000-0000-000000000000", Ordinal: 10},
	})
	if err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound from the batch, got %v", err)
	}

	ccs, err := s.ListChallengeCollections(ctx, teacher.ID)
	if err != nil {
		t.Fatalf("ListChallengeCollections: %v", err)
	}
	if ccs[0].Ordinal != cc.Ordinal {
		t.Errorf("ordinal = %d, want %d (unchanged — the batch should have rolled back)", ccs[0].Ordinal, cc.Ordinal)
	}
}

func TestReorderChallengeCollections_NotOwnerNotFound(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	owner := createTestUser(t, s, db, ctx, "collections-reorder-owner@example.com")
	other := createTestUser(t, s, db, ctx, "collections-reorder-other@example.com")

	cc, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: owner.ID, Title: "Owner's",
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, cc.ID, owner.ID) })

	err = s.ReorderChallengeCollections(ctx, other.ID, []store.CollectionReorderItem{
		{ID: cc.ID, Ordinal: 9},
	})
	if err != store.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound (non-owner can't reorder)", err)
	}
}

func TestListChallengesForViewer_GroupedByCollectionOrdinalThenCreatedAt(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "collections-ordering-teacher@example.com")

	ccB, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: teacher.ID, Title: "Collection B",
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection B: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, ccB.ID, teacher.ID) })

	// ccB (this teacher's first collection) got ordinal 0 automatically;
	// force ccA to sort before it with an explicit lower ordinal.
	lowerOrdinal := ccB.Ordinal - 1
	ccA, err := s.CreateChallengeCollection(ctx, store.CreateChallengeCollectionRequest{
		TeacherID: teacher.ID, Title: "Collection A", Ordinal: &lowerOrdinal,
	})
	if err != nil {
		t.Fatalf("CreateChallengeCollection A: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallengeCollection(ctx, ccA.ID, teacher.ID) })

	// ccA has a lower ordinal than ccB, so its challenges must appear first
	// regardless of creation order.
	chB, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "collections-ordering-b", Title: "In B", Difficulty: "easy", CollectionID: &ccB.ID,
	})
	if err != nil {
		t.Fatalf("CreateChallenge (B): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, chB.ID) })

	chA, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "collections-ordering-a", Title: "In A", Difficulty: "easy", CollectionID: &ccA.ID,
	})
	if err != nil {
		t.Fatalf("CreateChallenge (A): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, chA.ID) })

	chUncategorized, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "collections-ordering-none", Title: "Uncategorized", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge (uncategorized): %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, chUncategorized.ID) })

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

	idxA, idxB, idxNone := indexOf(chA.ID), indexOf(chB.ID), indexOf(chUncategorized.ID)
	if idxA >= idxB {
		t.Errorf("collection A (ordinal %d) must sort before collection B (ordinal %d): idxA=%d idxB=%d",
			ccA.Ordinal, ccB.Ordinal, idxA, idxB)
	}
	if idxB >= idxNone {
		t.Errorf("categorized challenges must sort before uncategorized ones: idxB=%d idxNone=%d", idxB, idxNone)
	}

	for _, it := range items {
		switch it.ID {
		case chA.ID:
			if it.CollectionTitle == nil || *it.CollectionTitle != "Collection A" {
				t.Errorf("chA CollectionTitle = %v, want %q", it.CollectionTitle, "Collection A")
			}
		case chB.ID:
			if it.CollectionTitle == nil || *it.CollectionTitle != "Collection B" {
				t.Errorf("chB CollectionTitle = %v, want %q", it.CollectionTitle, "Collection B")
			}
		case chUncategorized.ID:
			if it.CollectionTitle != nil {
				t.Errorf("uncategorized CollectionTitle = %v, want nil", it.CollectionTitle)
			}
		}
	}
}
