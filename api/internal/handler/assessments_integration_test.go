//go:build integration

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/caze/ascend/api/internal/auth"
	appmw "github.com/caze/ascend/api/internal/middleware"
	"github.com/caze/ascend/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestListAssessments(t *testing.T) {
	db := openHandlerTestDB(t)
	ctx := context.Background()
	s := store.New(db, nil)
	suffix := fmt.Sprint(time.Now().UnixNano())
	var users []string
	var listID string
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if listID != "" {
			if _, err := db.ExecContext(cleanupCtx, `DELETE FROM list_assessment_attempts WHERE revision_id IN (SELECT ar.id FROM list_assessment_revisions ar JOIN list_items li ON li.id=ar.list_item_id WHERE li.list_id=$1)`, listID); err != nil {
				t.Error(err)
			}
			if _, err := db.ExecContext(cleanupCtx, `DELETE FROM problem_lists WHERE id=$1`, listID); err != nil {
				t.Error(err)
			}
		}
		for _, id := range users {
			if _, err := db.ExecContext(cleanupCtx, `DELETE FROM users WHERE id=$1`, id); err != nil {
				t.Error(err)
			}
		}
	})
	createUser := func(name, role string) store.User {
		t.Helper()
		u, err := s.CreateUser(ctx, "assessment-"+name+"-"+suffix+"@example.com", "unused")
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, u.ID)
		if _, err := db.ExecContext(ctx, `UPDATE users SET role=$1 WHERE id=$2`, role, u.ID); err != nil {
			t.Fatal(err)
		}
		return u
	}
	mentor, other, student, peer := createUser("mentor", "teacher"), createUser("other", "teacher"), createUser("student", "student"), createUser("peer", "student")
	list, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: mentor.ID, Title: "Assessment fixture " + suffix})
	if err != nil {
		t.Fatal(err)
	}
	listID = list.ID
	item := func(title string) store.ListItem {
		t.Helper()
		it, err := s.CreateListItem(ctx, list.ID, mentor.ID, store.CreateListItemRequest{Title: title, Difficulty: "easy", Body: "Original context"})
		if err != nil {
			t.Fatal(err)
		}
		return it
	}
	choice, short, legacy := item("Choice"), item("Short"), item("Legacy")
	h, lh := NewAssessmentsHandler(s), NewListsHandler(s)
	r := chi.NewRouter()
	r.Use(appmw.NewPangolinAuth(s, "").Middleware, appmw.ViewAs)
	r.Get("/list-items/{id}/assessment", h.Get)
	r.With(auth.RequireAuthenticated, auth.RequireRole("teacher")).Get("/list-items/{id}/assessment/authoring", h.Authoring)
	r.With(auth.RequireAuthenticated, auth.RequireRole("teacher")).Post("/list-items/{id}/assessment", h.Save)
	r.With(auth.RequireAuthenticated).Post("/list-items/{id}/answers", h.Answer)
	r.With(auth.RequireAuthenticated).Get("/list-items/{id}/answers", h.Attempts)
	r.With(auth.RequireAuthenticated).Post("/list-items/{id}/complete", lh.CompleteItem)
	r.With(auth.RequireAuthenticated).Delete("/list-items/{id}/complete", lh.UncompleteItem)
	r.With(auth.RequireAuthenticated, auth.RequireRole("teacher")).Delete("/list-items/{id}", lh.DeleteItem)
	r.With(auth.RequireAuthenticated, auth.RequireRole("teacher")).Delete("/lists/{id}", lh.Delete)
	r.Get("/lists/{id}", lh.Get)
	server := httptest.NewServer(r)
	defer server.Close()
	base := os.Getenv("LIVE_ROOM_DEPLOYED_URL")
	if base == "" {
		base = server.URL
	}
	client := &http.Client{Timeout: 10 * time.Second}
	request := func(method, path, email, viewAs string, body any, want int, out any) string {
		t.Helper()
		var encoded []byte
		if body != nil {
			var err error
			encoded, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Remote-Email", email)
		req.Header.Set("X-View-As", viewAs)
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s: status=%d, want %d: %s", method, path, response.StatusCode, want, raw)
		}
		if out != nil {
			if err := json.Unmarshal(raw, out); err != nil {
				t.Fatal(err)
			}
		}
		return string(raw)
	}
	path := func(it store.ListItem) string { return "/list-items/" + it.ID }
	choiceInput := store.AssessmentInput{Kind: "multiple_choice", Prompt: "Pick the blue color", Options: []string{"Red", "Blue"}, AcceptedAnswers: []string{"Blue"}}
	shortInput := store.AssessmentInput{Kind: "short_answer", Prompt: "Give the phrase", AcceptedAnswers: []string{"Keep Spaces", "answer-key-private-canary"}}
	var c, q store.AssessmentAuthoring
	request(http.MethodPost, path(choice)+"/assessment", mentor.Email, "", choiceInput, http.StatusCreated, &c)
	request(http.MethodPost, path(short)+"/assessment", mentor.Email, "", shortInput, http.StatusCreated, &q)
	request(http.MethodGet, path(choice)+"/assessment", "", "", nil, http.StatusNotFound, nil)
	request(http.MethodGet, path(short)+"/assessment/authoring", other.Email, "", nil, http.StatusNotFound, nil)
	request(http.MethodGet, path(short)+"/assessment/authoring", student.Email, "", nil, http.StatusForbidden, nil)
	request(http.MethodGet, path(short)+"/assessment/authoring", mentor.Email, "student", nil, http.StatusForbidden, nil)
	answer := func(it store.ListItem, who store.User, rev, text string, want int) store.AssessmentAttempt {
		t.Helper()
		var attempt store.AssessmentAttempt
		raw := request(http.MethodPost, path(it)+"/answers", who.Email, "", map[string]string{"revision_id": rev, "answer": text}, want, &attempt)
		if strings.Contains(raw, "accepted_answers") || strings.Contains(raw, "answer-key-private-canary") {
			t.Fatal("answer response exposes private key")
		}
		return attempt
	}
	answer(choice, student, c.ID, "Blue", http.StatusNotFound)
	request(http.MethodPost, path(choice)+"/answers", "", "", map[string]string{"revision_id": c.ID, "answer": "Blue"}, http.StatusUnauthorized, nil)
	request(http.MethodPost, path(choice)+"/answers", mentor.Email, "student", map[string]string{"revision_id": c.ID, "answer": "Blue"}, http.StatusForbidden, nil)
	if _, err := s.UpdateProblemList(ctx, list.ID, mentor.ID, store.UpdateProblemListRequest{Title: list.Title, Published: true}); err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"", student.Email, other.Email} {
		raw := request(http.MethodGet, path(short)+"/assessment", email, "", nil, http.StatusOK, nil)
		if strings.Contains(raw, "accepted_answers") || strings.Contains(raw, "answer-key-private-canary") {
			t.Fatal("public question exposes key")
		}
		raw = request(http.MethodGet, "/lists/"+list.ID, email, "", nil, http.StatusOK, nil)
		if strings.Contains(raw, "accepted_answers") || strings.Contains(raw, "answer-key-private-canary") {
			t.Fatal("list payload exposes key")
		}
	}
	request(http.MethodPost, path(choice)+"/complete", student.Email, "", nil, http.StatusUnprocessableEntity, nil)
	request(http.MethodDelete, path(choice)+"/complete", student.Email, "", nil, http.StatusUnprocessableEntity, nil)
	for i := 0; i < 3; i++ {
		if answer(choice, student, c.ID, "Red", http.StatusCreated).Accepted {
			t.Fatal("wrong choice accepted")
		}
	}
	overview, err := s.TeacherStudentsOverview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range overview {
		if row.StudentID == student.ID {
			found = true
			if row.AssessmentStats.Attempts != 3 || row.AssessmentStats.RecurringDifficulties != 1 {
				t.Fatalf("wrong assessment difficulty: %+v", row.AssessmentStats)
			}
		}
	}
	if !found {
		t.Fatal("assessment-only student missing from dashboard")
	}
	if !answer(choice, student, c.ID, "Blue", http.StatusCreated).Accepted {
		t.Fatal("correct choice rejected")
	}
	var detail store.ProblemListDetail
	request(http.MethodGet, "/lists/"+list.ID, student.Email, "", nil, http.StatusOK, &detail)
	if detail.Items[0].Completed == nil || !*detail.Items[0].Completed {
		t.Fatal("accepted question not completed")
	}
	var unchanged store.AssessmentAuthoring
	request(http.MethodPost, path(choice)+"/assessment", mentor.Email, "", choiceInput, http.StatusCreated, &unchanged)
	if unchanged.ID != c.ID {
		t.Fatal("unchanged save resets completion")
	}
	if _, err := s.UpdateListItem(ctx, choice.ID, mentor.ID, store.UpdateListItemRequest{Title: choice.Title, Difficulty: choice.Difficulty, Body: "Unrelated context edit"}); err != nil {
		t.Fatal(err)
	}
	request(http.MethodGet, path(choice)+"/assessment/authoring", mentor.Email, "", nil, http.StatusOK, &unchanged)
	if unchanged.ID != c.ID || unchanged.Prompt != choiceInput.Prompt {
		t.Fatal("ordinary item edit changed grading revision")
	}
	choiceInput.Prompt = "Now pick red"
	choiceInput.AcceptedAnswers = []string{"Red"}
	var revised store.AssessmentAuthoring
	request(http.MethodPost, path(choice)+"/assessment", mentor.Email, "", choiceInput, http.StatusCreated, &revised)
	if revised.Revision != 2 || revised.ID == c.ID {
		t.Fatal("new question not versioned")
	}
	answer(choice, student, c.ID, "Blue", http.StatusConflict)
	request(http.MethodGet, "/lists/"+list.ID, student.Email, "", nil, http.StatusOK, &detail)
	if detail.Items[0].Completed == nil || *detail.Items[0].Completed {
		t.Fatal("old revision solves the new question")
	}
	var history []store.AssessmentAttempt
	raw := request(http.MethodGet, path(choice)+"/answers", student.Email, "", nil, http.StatusOK, &history)
	if len(history) != 4 || history[0].Exercise.Prompt != "Pick the blue color" || strings.Contains(raw, "accepted_answers") {
		t.Fatal("attempt history lost its original revision or leaked key")
	}
	request(http.MethodGet, path(choice)+"/answers", peer.Email, "", nil, http.StatusOK, &history)
	if len(history) != 0 {
		t.Fatal("peer received someone else's attempts")
	}
	answer(choice, student, revised.ID, strings.Repeat("a", 2001), http.StatusUnprocessableEntity)
	if !answer(short, peer, q.ID, "  keep\tSPACES  ", http.StatusCreated).Accepted {
		t.Fatal("short-answer normalization failed")
	}
	if answer(short, peer, q.ID, "unrelated", http.StatusCreated).Accepted {
		t.Fatal("wrong short answer accepted")
	}
	// A legacy checkbox must cease to count when automatic grading is added.
	if err := s.CompleteListItem(ctx, legacy.ID, student.ID); err != nil {
		t.Fatal(err)
	}
	request(http.MethodPost, path(legacy)+"/assessment", mentor.Email, "", shortInput, http.StatusCreated, nil)
	request(http.MethodGet, "/lists/"+list.ID, student.Email, "", nil, http.StatusOK, &detail)
	if detail.Items[2].Completed == nil || *detail.Items[2].Completed {
		t.Fatal("legacy checkbox bypassed automatic grading")
	}
	request(http.MethodDelete, path(choice), mentor.Email, "", nil, http.StatusConflict, nil)
	request(http.MethodDelete, "/lists/"+list.ID, mentor.Email, "", nil, http.StatusConflict, nil)
	overview, err = s.TeacherStudentsOverview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range overview {
		if row.StudentID == student.ID && (row.AssessmentStats.ItemsSolved != 0 || len(row.Completions) != 0) {
			t.Fatalf("stale completion in dashboard: %+v", row)
		}
		if row.StudentID == peer.ID && (row.AssessmentStats.ItemsSolved != 1 || len(row.Completions) != 1) {
			t.Fatalf("short answer not reflected in progress: %+v", row)
		}
	}
}
