package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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

// ImportChallengesRequest is the payload for POST /challenges/import — a
// batch of challenges, plus an optional collection every one of them gets
// linked to. CollectionTitle is batch-level (not per-challenge) because a
// whole import normally belongs to one collection.
type ImportChallengesRequest struct {
	TeacherID string
	// CollectionTitle, when non-empty, is matched case-insensitively against
	// this teacher's own collections; if none matches, a new one is created
	// in the same transaction. Every challenge in the batch is linked to it,
	// overriding any per-challenge CollectionID.
	CollectionTitle *string
	Challenges      []ImportChallengeRequest
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

// resolveOrCreateCollection finds teacherID's collection matching title
// case-insensitively, or creates one (ordinal = next available among the
// teacher's own collections) if none matches — both inside the caller's
// transaction, so a brand-new collection is only committed along with the
// batch that named it.
func resolveOrCreateCollection(ctx context.Context, tx *sql.Tx, teacherID, title string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM challenge_collections WHERE teacher_id = $1 AND LOWER(title) = LOWER($2)`,
		teacherID, title,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("find challenge collection: %w", err)
	}

	err = tx.QueryRowContext(ctx,
		`INSERT INTO challenge_collections (teacher_id, title, ordinal)
		 VALUES ($1, $2, (SELECT COALESCE(MAX(ordinal), -1) + 1 FROM challenge_collections WHERE teacher_id = $1))
		 RETURNING id`,
		teacherID, title,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create challenge collection: %w", err)
	}
	return id, nil
}

// ImportChallenges creates every challenge and its full test suite in a
// single transaction: any failure — a duplicate slug anywhere in the batch
// or against the database, or any other insert error — rolls back the whole
// request, so nothing is persisted half-imported. Mirrors ImportProblemList.
func (s *Store) ImportChallenges(ctx context.Context, req ImportChallengesRequest) ([]ChallengeWithTestCases, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Batch-level collection: resolved/created once, then applied to every
	// challenge below, overriding any per-challenge CollectionID.
	var batchCollectionID *string
	if req.CollectionTitle != nil {
		if title := strings.TrimSpace(*req.CollectionTitle); title != "" {
			id, err := resolveOrCreateCollection(ctx, tx, req.TeacherID, title)
			if err != nil {
				return nil, err
			}
			batchCollectionID = &id
		}
	}

	results := make([]ChallengeWithTestCases, 0, len(req.Challenges))
	for i, item := range req.Challenges {
		timeLimitMs := item.TimeLimitMs
		if timeLimitMs == 0 {
			timeLimitMs = 2000
		}
		memLimitMb := item.MemoryLimitMb
		if memLimitMb == 0 {
			memLimitMb = 256
		}

		collectionID := item.CollectionID
		if batchCollectionID != nil {
			collectionID = batchCollectionID
		} else if err := validateCollectionID(ctx, tx, collectionID); err != nil {
			return nil, fmt.Errorf("challenge %d: %w", i, err)
		}

		row := tx.QueryRowContext(ctx,
			`INSERT INTO challenges (slug, title, description, difficulty, time_limit_ms, memory_limit_mb, notes, starter_code, language, sql_schema, collection_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			 RETURNING `+challengeColumns,
			item.Slug, item.Title, item.Description, item.Difficulty, timeLimitMs, memLimitMb, item.Notes, item.StarterCode,
			item.Language, item.SQLSchema, collectionID)
		c, err := scanChallenge(row)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
				return nil, &ImportConflictError{Index: i}
			}
			return nil, fmt.Errorf("insert challenge %d: %w", i, err)
		}

		tcs := make([]TestCase, 0, len(item.TestCases))
		for j, tc := range item.TestCases {
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
