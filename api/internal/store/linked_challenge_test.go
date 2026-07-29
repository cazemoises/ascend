//go:build integration

package store_test

import (
	"context"
	"errors"
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

func TestImportProblemList_LinkedChallengeSlugResolves(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "import-linked-slug-teacher@example.com")

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "import-linked-slug-challenge", Title: "Import Linked Slug Challenge", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	slug := ch.Slug
	detail, err := s.ImportProblemList(ctx, store.ImportProblemListRequest{
		TeacherID: teacher.ID,
		Title:     "Import Linked Slug List",
		Items: []store.CreateListItemRequest{
			{Title: "Plain item", Difficulty: "easy", Body: "b"},
			{Title: "Linked item", Difficulty: "easy", Body: "b", LinkedChallengeSlug: &slug},
		},
	})
	if err != nil {
		t.Fatalf("ImportProblemList: %v", err)
	}
	t.Cleanup(func() { s.DeleteProblemList(ctx, detail.ID, teacher.ID) })

	if len(detail.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(detail.Items))
	}
	if detail.Items[0].LinkedChallengeID != nil {
		t.Errorf("plain item LinkedChallengeID = %v, want nil", detail.Items[0].LinkedChallengeID)
	}
	if detail.Items[1].LinkedChallengeID == nil || *detail.Items[1].LinkedChallengeID != ch.ID {
		t.Errorf("linked item LinkedChallengeID = %v, want %q (resolved from slug %q)",
			detail.Items[1].LinkedChallengeID, ch.ID, slug)
	}

	// Confirm the resolved id was actually persisted, not just returned.
	var persistedID *string
	if err := db.QueryRowContext(ctx,
		`SELECT linked_challenge_id FROM list_items WHERE id = $1`, detail.Items[1].ID,
	).Scan(&persistedID); err != nil {
		t.Fatalf("query persisted linked_challenge_id: %v", err)
	}
	if persistedID == nil || *persistedID != ch.ID {
		t.Errorf("persisted linked_challenge_id = %v, want %q", persistedID, ch.ID)
	}
}

func TestImportProblemList_LinkedChallengeSlugNotFoundRollsBackEverything(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()
	teacher := createTestUser(t, s, db, ctx, "import-linked-slug-notfound-teacher@example.com")

	bogusSlug := "does-not-exist-slug"
	const title = "Import Linked Slug Not Found List"
	_, err := s.ImportProblemList(ctx, store.ImportProblemListRequest{
		TeacherID: teacher.ID,
		Title:     title,
		Items: []store.CreateListItemRequest{
			{Title: "Good item", Difficulty: "easy", Body: "b"},
			{Title: "Bad item", Difficulty: "easy", Body: "b", LinkedChallengeSlug: &bogusSlug},
		},
	})

	var notFoundErr *store.ImportChallengeSlugNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("err = %v, want *ImportChallengeSlugNotFoundError", err)
	}
	if notFoundErr.Index != 1 || notFoundErr.Slug != bogusSlug {
		t.Errorf("notFoundErr = %+v, want Index=1 Slug=%q", notFoundErr, bogusSlug)
	}

	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM problem_lists WHERE teacher_id = $1 AND title = $2`,
		teacher.ID, title).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("persisted list count = %d, want 0 (full rollback, including the good item)", count)
	}
}
