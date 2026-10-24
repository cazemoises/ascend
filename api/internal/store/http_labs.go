package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/caze/ascend/pkg/httplab"
)

type HTTPLabInput struct {
	StarterCode   string                  `json:"starter_code"`
	TimeLimitMs   int                     `json:"time_limit_ms"`
	MemoryLimitMb int                     `json:"memory_limit_mb"`
	TestCases     []CreateTestCaseRequest `json:"test_cases"`
}

func (in HTTPLabInput) Validate() error {
	if len(in.StarterCode) == 0 || len(in.StarterCode) > 65536 {
		return fmt.Errorf("main.py requires 1–65536 bytes")
	}
	if in.TimeLimitMs < 1000 || in.TimeLimitMs > 30000 || in.MemoryLimitMb < 64 || in.MemoryLimitMb > 512 {
		return fmt.Errorf("limits: 1000–30000 ms, 64–512 MiB")
	}
	if len(in.TestCases) < 1 || len(in.TestCases) > 30 {
		return fmt.Errorf("provide 1–30 HTTP tests")
	}
	for _, tc := range in.TestCases {
		if _, err := httplab.ParseRequest(tc.Input); err != nil {
			return err
		}
		if _, err := httplab.ParseResponse(tc.ExpectedOutput); err != nil {
			return err
		}
	}
	return nil
}

// CreateHTTPLab links a fully configured laboratory atomically, without changing
// the list's editorial state or replacing an existing exercise.
func (s *Store) CreateHTTPLab(ctx context.Context, itemID, userID string, in HTTPLabInput) (string, error) {
	if err := in.Validate(); err != nil {
		return "", err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var listID, title, body, difficulty string
	var linked sql.NullString
	var assessment bool
	err = tx.QueryRowContext(ctx, `SELECT li.list_id,li.title,li.body,li.difficulty,li.linked_challenge_id,EXISTS(SELECT 1 FROM list_assessment_revisions ar WHERE ar.list_item_id=li.id) FROM list_items li JOIN problem_lists pl ON pl.id=li.list_id WHERE li.id=$1 AND pl.teacher_id=$2 FOR UPDATE OF li`, itemID, userID).Scan(&listID, &title, &body, &difficulty, &linked, &assessment)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("lock lab item: %w", err)
	}
	if linked.Valid || assessment {
		return "", ErrConflict
	}
	var id string
	err = tx.QueryRowContext(ctx, `INSERT INTO challenges(slug,title,description,difficulty,time_limit_ms,memory_limit_mb,starter_code,language,lab_owner_id,lab_list_id) VALUES ('http-lab-'||$1,$2,$3,$4,$5,$6,$7,'fastapi',$8,$9) RETURNING id`, itemID, title, body, difficulty, in.TimeLimitMs, in.MemoryLimitMb, in.StarterCode, userID, listID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create HTTP lab: %w", err)
	}
	for i, tc := range in.TestCases {
		_, err = tx.ExecContext(ctx, `INSERT INTO test_cases(challenge_id,input,expected_output,is_sample,ordinal) VALUES($1,$2,$3,$4,$5)`, id, tc.Input, tc.ExpectedOutput, tc.IsSample, i)
		if err != nil {
			return "", fmt.Errorf("create HTTP test: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE list_items SET linked_challenge_id=$2,updated_at=now() WHERE id=$1`, itemID, id); err != nil {
		return "", fmt.Errorf("link HTTP lab: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

// CanAccessChallenge leaves legacy challenge permissions unchanged. Labs inherit
// visibility from their original list and restrict all authoring to its mentor.
func (s *Store) CanAccessChallenge(ctx context.Context, id, userID string, teacher, write bool) (bool, error) {
	var ok bool
	err := s.db.QueryRowContext(ctx, `SELECT CASE WHEN c.language IS DISTINCT FROM 'fastapi' THEN true ELSE
 COALESCE(c.lab_owner_id=NULLIF($2,'')::uuid AND $3,false) OR (NOT $4 AND EXISTS(SELECT 1 FROM problem_lists pl WHERE pl.id=c.lab_list_id AND pl.published)) END FROM challenges c WHERE c.id=$1`, id, userID, teacher, write).Scan(&ok)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return ok, err
}

func (s *Store) IsHTTPLab(ctx context.Context, id string) (bool, error) {
	var lab bool
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(language='fastapi',false) FROM challenges WHERE id=$1`, id).Scan(&lab)
	return lab, err
}

// PrepareHTTPFeedback is a response-boundary guard, including older persisted
// results. Stateful requests after a hidden request can reveal its data even
// when the later request is labelled public. Only a public prefix is safe.
func (s *Store) PrepareHTTPFeedback(ctx context.Context, sub *Submission) error {
	if sub.HTTPExploration {
		sub.HTTPFeedbackSafe = true
		return nil
	}
	var public, hidden bool
	err := s.db.QueryRowContext(ctx, `WITH ordered AS (
 SELECT *,row_number() OVER (ORDER BY ordinal)-1 AS position FROM test_cases WHERE challenge_id=$1
 ) SELECT
 EXISTS(SELECT 1 FROM ordered tc WHERE tc.position=$4 AND tc.is_sample
 AND tc.input=$2 AND tc.expected_output=$3
 AND NOT EXISTS(SELECT 1 FROM test_cases h WHERE h.challenge_id=$1 AND NOT h.is_sample AND h.ordinal<=tc.ordinal)),
 EXISTS(SELECT 1 FROM test_cases WHERE challenge_id=$1 AND NOT is_sample)`, sub.ChallengeID, sub.FailedInput, sub.ExpectedOutput, sub.PassedCount).Scan(&public, &hidden)
	if err != nil {
		return fmt.Errorf("check HTTP feedback visibility: %w", err)
	}
	sub.HTTPFeedbackSafe = !hidden
	if hidden {
		sub.Stderr = nil
	}
	if sub.Status != "wrong_answer" {
		sub.ExpectedOutput = nil
		sub.FailedInput = nil
		if hidden {
			sub.Stdout = nil
		}
		return nil
	}
	if sub.FailedIsSample == nil || !*sub.FailedIsSample || !public {
		sub.Stdout = nil
		sub.ExpectedOutput = nil
		sub.FailedInput = nil
		value := false
		sub.FailedIsSample = &value
		return nil
	}
	if sub.Stdout != nil && sub.ExpectedOutput != nil {
		comparison, err := httplab.Compare(*sub.Stdout, *sub.ExpectedOutput)
		if err == nil {
			sub.HTTPDebug = comparison
		} else {
			sub.Stdout = nil
			sub.ExpectedOutput = nil
			sub.Stderr = nil
		}
	}
	return nil
}
