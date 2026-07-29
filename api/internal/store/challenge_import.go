package store

import (
	"context"
	"fmt"

	"github.com/lib/pq"
)

// ImportTestCaseRequest mirrors CreateTestCaseRequest but adds an optional
// explicit Ordinal: nil means "use this test case's position in the array",
// matching ReplaceTestCases' default, while a non-nil value lets the caller
// override it (e.g. re-importing a suite that was reordered elsewhere).
type ImportTestCaseRequest struct {
	Input          string
	ExpectedOutput string
	IsSample       bool
	Ordinal        *int
	OrderMatters   bool
}

// ImportChallengeRequest is one challenge in a bulk import: the same fields
// CreateChallenge accepts, plus its full test suite.
type ImportChallengeRequest struct {
	CreateChallengeRequest
	TestCases []ImportTestCaseRequest
}

// ChallengeWithTestCases is the import response shape for one challenge:
// every field GetChallengeDetail's Challenge embed carries, plus the full
// test suite (not just the public samples GetChallengeDetail exposes to
// students) — the teacher who just created it needs to see everything that
// was actually persisted.
type ChallengeWithTestCases struct {
	Challenge
	TestCases []TestCase `json:"test_cases"`
}

// ImportConflictError reports that the challenge at Index collided with an
// existing slug — either another row already in the database, or an earlier
// item in the same import batch (Postgres's unique constraint catches both
// identically, since earlier items in the transaction are already inserted,
// just not yet committed).
type ImportConflictError struct {
	Index int
}

func (e *ImportConflictError) Error() string {
	return fmt.Sprintf("challenge %d: slug already exists", e.Index)
}

// ImportChallenges creates every challenge and its full test suite in a
// single transaction: any failure — a duplicate slug anywhere in the batch
// or against the database, or any other insert error — rolls back the whole
// request, so nothing is persisted half-imported. Mirrors ImportProblemList.
func (s *Store) ImportChallenges(ctx context.Context, reqs []ImportChallengeRequest) ([]ChallengeWithTestCases, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	results := make([]ChallengeWithTestCases, 0, len(reqs))
	for i, req := range reqs {
		timeLimitMs := req.TimeLimitMs
		if timeLimitMs == 0 {
			timeLimitMs = 2000
		}
		memLimitMb := req.MemoryLimitMb
		if memLimitMb == 0 {
			memLimitMb = 256
		}

		if err := validateCollectionID(ctx, tx, req.CollectionID); err != nil {
			return nil, fmt.Errorf("challenge %d: %w", i, err)
		}

		row := tx.QueryRowContext(ctx,
			`INSERT INTO challenges (slug, title, description, difficulty, time_limit_ms, memory_limit_mb, notes, starter_code, language, sql_schema, collection_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			 RETURNING `+challengeColumns,
			req.Slug, req.Title, req.Description, req.Difficulty, timeLimitMs, memLimitMb, req.Notes, req.StarterCode,
			req.Language, req.SQLSchema, req.CollectionID)
		c, err := scanChallenge(row)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
				return nil, &ImportConflictError{Index: i}
			}
			return nil, fmt.Errorf("insert challenge %d: %w", i, err)
		}

		tcs := make([]TestCase, 0, len(req.TestCases))
		for j, tc := range req.TestCases {
			ordinal := j
			if tc.Ordinal != nil {
				ordinal = *tc.Ordinal
			}
			var t TestCase
			err := tx.QueryRowContext(ctx,
				`INSERT INTO test_cases (challenge_id, input, expected_output, is_sample, order_matters, ordinal)
				 VALUES ($1, $2, $3, $4, $5, $6)
				 RETURNING id, challenge_id, input, expected_output, is_sample, ordinal, order_matters`,
				c.ID, tc.Input, tc.ExpectedOutput, tc.IsSample, tc.OrderMatters, ordinal,
			).Scan(&t.ID, &t.ChallengeID, &t.Input, &t.ExpectedOutput, &t.IsSample, &t.Ordinal, &t.OrderMatters)
			if err != nil {
				return nil, fmt.Errorf("insert test case %d for challenge %d: %w", j, i, err)
			}
			tcs = append(tcs, t)
		}

		results = append(results, ChallengeWithTestCases{Challenge: c, TestCases: tcs})
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return results, nil
}
