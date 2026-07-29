//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/caze/ascend/api/internal/store"
)

func TestGetProblemListDetail_LinkedItemCompletedFromChallengeSolved(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "linked-item-solved-teacher@example.com")
	student := createTestUser(t, s, db, ctx, "linked-item-solved-student@example.com")

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "linked-item-challenge", Title: "Linked Item Challenge", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id = $1`, ch.ID)
		s.DeleteChallenge(ctx, ch.ID)
	})

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Linked"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })
	if _, err := s.UpdateProblemList(ctx, list.ID, teacher.ID, store.UpdateProblemListRequest{Title: list.Title, Published: true}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
		Title: "Solve it", Difficulty: "easy", Body: "b", LinkedChallengeID: &ch.ID,
	})
	if err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}
	if item.LinkedChallengeID == nil || *item.LinkedChallengeID != ch.ID {
		t.Fatalf("LinkedChallengeID not persisted: %+v", item.LinkedChallengeID)
	}

	t.Run("not yet solved: completed=false", func(t *testing.T) {
		detail, err := s.GetProblemListDetail(ctx, list.ID, student.ID, "student")
		if err != nil {
			t.Fatalf("GetProblemListDetail: %v", err)
		}
		got := detail.Items[0]
		if got.Completed == nil || *got.Completed {
			t.Errorf("completed = %v, want false (student hasn't solved the challenge yet)", got.Completed)
		}
		if got.ChallengeTitle == nil || *got.ChallengeTitle != ch.Title {
			t.Errorf("challenge_title = %v, want %q", got.ChallengeTitle, ch.Title)
		}
		if got.ChallengeSlug == nil || *got.ChallengeSlug != ch.Slug {
			t.Errorf("challenge_slug = %v, want %q", got.ChallengeSlug, ch.Slug)
		}
	})

	// Same rule as the [CONCLUÍDO] badge: accepted, non-test-run submission.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO submissions (challenge_id, user_id, language, source_code, status, is_test_run)
		 VALUES ($1, $2, 'go', 'x', 'accepted', false)`, ch.ID, student.ID); err != nil {
		t.Fatalf("insert accepted submission: %v", err)
	}

	t.Run("solved: completed=true automatically, without calling /complete", func(t *testing.T) {
		detail, err := s.GetProblemListDetail(ctx, list.ID, student.ID, "student")
		if err != nil {
			t.Fatalf("GetProblemListDetail: %v", err)
		}
		got := detail.Items[0]
		if got.Completed == nil || !*got.Completed {
			t.Errorf("completed = %v, want true (student solved the linked challenge)", got.Completed)
		}

		var count int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM list_item_completions WHERE list_item_id = $1`, item.ID).Scan(&count); err != nil {
			t.Fatalf("count completions: %v", err)
		}
		if count != 0 {
			t.Errorf("list_item_completions rows = %d, want 0 (completion is derived, never self-declared)", count)
		}
	})
}

func TestCompleteListItem_RejectsLinkedItem(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "linked-item-reject-teacher@example.com")
	student := createTestUser(t, s, db, ctx, "linked-item-reject-student@example.com")

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "linked-item-reject-challenge", Title: "Reject Challenge", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Reject"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })
	if _, err := s.UpdateProblemList(ctx, list.ID, teacher.ID, store.UpdateProblemListRequest{Title: list.Title, Published: true}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	item, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
		Title: "Solve it", Difficulty: "easy", Body: "b", LinkedChallengeID: &ch.ID,
	})
	if err != nil {
		t.Fatalf("CreateListItem: %v", err)
	}

	if err := s.CompleteListItem(ctx, item.ID, student.ID); err != store.ErrLinkedItem {
		t.Errorf("CompleteListItem err = %v, want ErrLinkedItem", err)
	}
	if err := s.UncompleteListItem(ctx, item.ID, student.ID); err != store.ErrLinkedItem {
		t.Errorf("UncompleteListItem err = %v, want ErrLinkedItem", err)
	}
}

// TestCreateListItem_LinkedChallengeNotFound and the mixed-list regression
// test below cover the two remaining explicit requirements: a bad
// linked_challenge_id is rejected, and an item without one keeps working
// exactly as before in the same list as a linked item.
func TestCreateListItem_LinkedChallengeNotFound(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "linked-item-notfound-teacher@example.com")

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Bad Link"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })

	bogusID := "00000000-0000-0000-0000-000000000000"
	_, err = s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
		Title: "Solve it", Difficulty: "easy", Body: "b", LinkedChallengeID: &bogusID,
	})
	if err != store.ErrLinkedChallengeNotFound {
		t.Errorf("err = %v, want ErrLinkedChallengeNotFound", err)
	}
}

func TestGetProblemListDetail_MixedLinkedAndSelfDeclaredItems(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "linked-item-mixed-teacher@example.com")
	student := createTestUser(t, s, db, ctx, "linked-item-mixed-student@example.com")

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "linked-item-mixed-challenge", Title: "Mixed Challenge", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: teacher.ID, Title: "Mixed"})
	if err != nil {
		t.Fatalf("CreateProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, list.ID, teacher.ID) })
	if _, err := s.UpdateProblemList(ctx, list.ID, teacher.ID, store.UpdateProblemListRequest{Title: list.Title, Published: true}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	linkedItem, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
		Title: "Linked", Difficulty: "easy", Body: "b", LinkedChallengeID: &ch.ID,
	})
	if err != nil {
		t.Fatalf("CreateListItem (linked): %v", err)
	}
	plainItem, err := s.CreateListItem(ctx, list.ID, teacher.ID, store.CreateListItemRequest{
		Title: "Plain", Difficulty: "easy", Body: "b",
	})
	if err != nil {
		t.Fatalf("CreateListItem (plain): %v", err)
	}

	// Regression: the plain item's self-declared checkbox still works
	// exactly as before, independent of the linked item in the same list.
	if err := s.CompleteListItem(ctx, plainItem.ID, student.ID); err != nil {
		t.Fatalf("CompleteListItem (plain item): %v", err)
	}

	detail, err := s.GetProblemListDetail(ctx, list.ID, student.ID, "student")
	if err != nil {
		t.Fatalf("GetProblemListDetail: %v", err)
	}

	var gotLinked, gotPlain store.ListItem
	for _, it := range detail.Items {
		switch it.ID {
		case linkedItem.ID:
			gotLinked = it
		case plainItem.ID:
			gotPlain = it
		}
	}

	if gotLinked.Completed == nil || *gotLinked.Completed {
		t.Errorf("linked item completed = %v, want false (challenge not solved)", gotLinked.Completed)
	}
	if gotLinked.LinkedChallengeID == nil || *gotLinked.LinkedChallengeID != ch.ID {
		t.Errorf("linked item LinkedChallengeID = %v, want %q", gotLinked.LinkedChallengeID, ch.ID)
	}
	if gotPlain.Completed == nil || !*gotPlain.Completed {
		t.Errorf("plain item completed = %v, want true (self-declared via CompleteListItem)", gotPlain.Completed)
	}
	if gotPlain.LinkedChallengeID != nil {
		t.Errorf("plain item LinkedChallengeID = %v, want nil", gotPlain.LinkedChallengeID)
	}
}
