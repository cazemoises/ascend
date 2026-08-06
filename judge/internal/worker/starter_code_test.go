package worker

import "testing"

func TestStarterCodeFor_PicksMatchingLanguageFromMap(t *testing.T) {
	c := challengeRecord{
		StarterCodeByLanguage: map[string]string{
			"python":     "py stub\n[[ASCEND::RUNNER]]\npy harness",
			"javascript": "js stub\n[[ASCEND::RUNNER]]\njs harness",
		},
	}

	got := c.starterCodeFor("javascript")

	want := "js stub\n[[ASCEND::RUNNER]]\njs harness"
	if got != want {
		t.Errorf("starterCodeFor(javascript) = %q, want %q", got, want)
	}
}

// This is the regression test for the core bug: a challenge with a harness
// authored for python only must never splice that python harness under a
// javascript submission just because a map entry exists for a different
// language.
func TestStarterCodeFor_MissingLanguageInMapReturnsEmpty(t *testing.T) {
	c := challengeRecord{
		StarterCodeByLanguage: map[string]string{
			"python": "py stub\n[[ASCEND::RUNNER]]\npy harness",
		},
	}

	got := c.starterCodeFor("javascript")

	if got != "" {
		t.Errorf("starterCodeFor(javascript) = %q, want empty (no harness authored for this language)", got)
	}
}

func TestStarterCodeFor_FallsBackToLegacyStarterCodeWhenMapEmpty(t *testing.T) {
	c := challengeRecord{
		StarterCode: "legacy stub\n[[ASCEND::RUNNER]]\nlegacy harness",
	}

	got := c.starterCodeFor("python")

	want := "legacy stub\n[[ASCEND::RUNNER]]\nlegacy harness"
	if got != want {
		t.Errorf("starterCodeFor(python) = %q, want %q", got, want)
	}
}

func TestDecodeStarterCodeByLanguage_ParsesJSONObject(t *testing.T) {
	got, err := decodeStarterCodeByLanguage([]byte(`{"python": "a", "go": "b"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["python"] != "a" || got["go"] != "b" || len(got) != 2 {
		t.Errorf("got %#v, want map[python:a go:b]", got)
	}
}

// Empty column value (no per-language harness authored yet) must decode to
// an empty map, not an error — this is the common case for every challenge
// created before this column existed.
func TestDecodeStarterCodeByLanguage_EmptyInputReturnsEmptyMap(t *testing.T) {
	got, err := decodeStarterCodeByLanguage(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %#v, want empty map", got)
	}
}
