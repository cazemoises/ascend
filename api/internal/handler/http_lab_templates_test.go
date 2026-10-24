package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/go-chi/chi/v5"
)

func TestHTTPSequenceCatalog(t *testing.T) {
	r := chi.NewRouter()
	h := NewListsHandler(nil)
	r.With(auth.RequireAuthenticated, auth.RequireRole("teacher")).Get("/sequence", h.HTTPSequence)
	for _, tc := range []struct {
		role string
		want int
	}{{"", 401}, {"student", 403}, {"teacher", 200}} {
		t.Run(tc.role, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/sequence", nil)
			if tc.role != "" {
				req = withClaims(req, auth.Claims{UserID: "fixture", Role: tc.role})
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d", w.Code, tc.want)
			}
			if tc.want != 200 {
				if bytes.Contains(w.Body.Bytes(), []byte("starter_code")) {
					t.Fatal("catalog exposed")
				}
				return
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("catalog must not be cached")
			}
		})
	}
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
