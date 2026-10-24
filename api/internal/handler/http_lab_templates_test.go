package handler

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caze/ascend/api/internal/auth"
)

// Administrative package stays outside the production binary and student bundle.
//
//go:embed activity_templates/fastapi-v1.json
var fastAPISequence []byte

func TestHTTPSequenceCatalog(t *testing.T) {
	var body importListBody
	if err := json.Unmarshal(fastAPISequence, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 6 {
		t.Fatalf("expected six activities: %d", len(body.Items))
	}
	for _, it := range body.Items {
		if msg, ok := it.validate(); !ok {
			t.Fatal(msg)
		}
		if it.HTTPLab == nil {
			t.Fatal("missing lab")
		}
		if err := it.HTTPLab.Validate(); err != nil {
			t.Fatal(err)
		}
		public, hidden := 0, 0
		for _, tc := range it.HTTPLab.TestCases {
			if tc.IsSample {
				if hidden > 0 {
					t.Fatal("public case after hidden case")
				}
				public++
			} else {
				hidden++
			}
		}
		if public == 0 || hidden == 0 {
			t.Fatal("each activity needs public and hidden cases")
		}
	}
}

func TestImportHTTPLabRejectsInvalidBeforeWriting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*importListBody)
	}{
		{"conflicting link", func(b *importListBody) { v := "existing"; b.Items[0].LinkedChallengeSlug = &v }},
		{"invalid request", func(b *importListBody) {
			b.Items[0].HTTPLab.TestCases[0].Input = `{"method":"GET","path":"https://example.com"}`
		}},
		{"invalid limits", func(b *importListBody) { b.Items[0].HTTPLab.MemoryLimitMb = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body importListBody
			if err := json.Unmarshal(fastAPISequence, &body); err != nil {
				t.Fatal(err)
			}
			tc.change(&body)
			data, _ := json.Marshal(body)
			req := withClaims(httptest.NewRequest("POST", "/lists/import", bytes.NewReader(data)), auth.Claims{UserID: "mentor", Role: "teacher"})
			w := httptest.NewRecorder()
			newListsTestRouter().ServeHTTP(w, req)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCreateHTTPItemRejectsInvalidBeforeWriting(t *testing.T) {
	for _, payload := range []string{
		`{"title":"HTTP","body":"Statement","difficulty":"easy","http_lab":{}}`,
		`{"title":"HTTP","body":"Statement","difficulty":"easy","linked_challenge_id":"existing","http_lab":{}}`,
	} {
		req := withClaims(httptest.NewRequest("POST", "/lists/fixture/items", bytes.NewBufferString(payload)), auth.Claims{UserID: "mentor", Role: "teacher"})
		response := httptest.NewRecorder()
		NewListsHandler(nil).CreateItem(response, req)
		if response.Code != 422 {
			t.Fatalf("invalid HTTP config must fail before persistence: %d", response.Code)
		}
	}
	req := withClaims(httptest.NewRequest("PATCH", "/list-items/fixture", bytes.NewBufferString(`{"title":"HTTP","body":"Statement","difficulty":"easy","http_lab":{}}`)), auth.Claims{UserID: "mentor", Role: "teacher"})
	response := httptest.NewRecorder()
	NewListsHandler(nil).UpdateItem(response, req)
	if response.Code != 422 {
		t.Fatalf("HTTP configuration updates must be explicit: %d", response.Code)
	}
}
