//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/caze/ascend/api/internal/store"
)

func TestCreateChallenge_PersistsStarterCodeByLanguage(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	byLang := store.StarterCodeByLanguageMap{
		"python":     "py stub\n[[ASCEND::RUNNER]]\npy harness",
		"javascript": "js stub\n[[ASCEND::RUNNER]]\njs harness",
	}
	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "starter-by-lang-test", Title: "Starter By Lang", Difficulty: "easy",
		StarterCodeByLanguage: byLang,
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	if ch.StarterCodeByLanguage["python"] != byLang["python"] || ch.StarterCodeByLanguage["javascript"] != byLang["javascript"] {
		t.Errorf("StarterCodeByLanguage = %+v, want %+v", ch.StarterCodeByLanguage, byLang)
	}

	got, err := s.GetChallenge(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetChallenge: %v", err)
	}
	if got.StarterCodeByLanguage["python"] != byLang["python"] || got.StarterCodeByLanguage["javascript"] != byLang["javascript"] {
		t.Errorf("GetChallenge StarterCodeByLanguage = %+v, want %+v", got.StarterCodeByLanguage, byLang)
	}
}

func TestCreateChallenge_StarterCodeByLanguageDefaultsNil(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db, nil)
	ctx := context.Background()

	ch, err := s.CreateChallenge(ctx, store.CreateChallengeRequest{
		Slug: "starter-by-lang-nil-test", Title: "No Starter By Lang", Difficulty: "easy",
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { s.DeleteChallenge(ctx, ch.ID) })

	if len(ch.StarterCodeByLanguage) != 0 {
		t.Errorf("StarterCodeByLanguage = %+v, want empty", ch.StarterCodeByLanguage)
	}
}
