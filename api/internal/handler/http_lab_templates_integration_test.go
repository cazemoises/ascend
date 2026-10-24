//go:build integration

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caze/ascend/api/internal/auth"
	"github.com/caze/ascend/api/internal/store"
)

func TestHTTPSequenceImport(t *testing.T) {
	db := openHandlerTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s := store.New(db, nil)
	owner, err := s.CreateUser(ctx, "sequence-import@example.com", "unused")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.ExecContext(ctx, `UPDATE list_items SET linked_challenge_id=NULL WHERE list_id IN (SELECT id FROM problem_lists WHERE teacher_id=$1)`, owner.ID)
		db.ExecContext(ctx, `DELETE FROM challenges WHERE lab_owner_id=$1`, owner.ID)
		db.ExecContext(ctx, `DELETE FROM problem_lists WHERE teacher_id=$1`, owner.ID)
		db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, owner.ID)
	}()
	h := NewListsHandler(s)
	call := func(payload []byte, want int) []byte {
		t.Helper()
		req := withClaims(httptest.NewRequest("POST", "/lists/import", bytes.NewReader(payload)), auth.Claims{UserID: owner.ID, Role: "teacher"})
		w := httptest.NewRecorder()
		h.Import(w, req)
		if w.Code != want {
			t.Fatalf("got %d want %d: %s", w.Code, want, w.Body.String())
		}
		return w.Body.Bytes()
	}
	var detail store.ProblemListDetail
	if err := json.Unmarshal(call(fastAPISequence, 201), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Published || len(detail.Items) != 6 {
		t.Fatal("must create complete draft")
	}
	for _, it := range detail.Items {
		if it.LinkedChallengeID == nil {
			t.Fatal("lab not linked")
		}
		allowed, err := s.CanAccessChallenge(ctx, *it.LinkedChallengeID, "", false, false)
		if err != nil || allowed {
			t.Fatalf("draft visible: %v %v", allowed, err)
		}
		allowed, err = s.CanAccessChallenge(ctx, *it.LinkedChallengeID, owner.ID, true, true)
		if err != nil || !allowed {
			t.Fatalf("owner cannot access: %v %v", allowed, err)
		}
		var public, hidden int
		if err = db.QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE is_sample), count(*) FILTER(WHERE NOT is_sample) FROM test_cases WHERE challenge_id=$1`, *it.LinkedChallengeID).Scan(&public, &hidden); err != nil {
			t.Fatal(err)
		}
		if public == 0 || hidden == 0 {
			t.Fatal("incomplete cases")
		}
	}
	// Fail after five labs have been inserted: all new rows must roll back.
	var body importListBody
	json.Unmarshal(fastAPISequence, &body)
	last := len(body.Items) - 1
	missing := "sequence-does-not-exist"
	body.Items[last].HTTPLab = nil
	body.Items[last].LinkedChallengeSlug = &missing
	payload, _ := json.Marshal(body)
	call(payload, 422)
	var lists, labs int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM problem_lists WHERE teacher_id=$1`, owner.ID).Scan(&lists); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM challenges WHERE lab_owner_id=$1`, owner.ID).Scan(&labs); err != nil {
		t.Fatal(err)
	}
	if lists != 1 || labs != 6 {
		t.Fatalf("partial import persisted: lists=%d labs=%d", lists, labs)
	}
}
