package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type Challenge struct {
	ID            string    `json:"id"`
	Slug          string    `json:"slug"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Difficulty    string    `json:"difficulty"`
	TimeLimitMs   int       `json:"time_limit_ms"`
	MemoryLimitMb int       `json:"memory_limit_mb"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateChallengeRequest struct {
	Slug          string
	Title         string
	Description   string
	Difficulty    string
	TimeLimitMs   int
	MemoryLimitMb int
}

type TestCase struct {
	ID             string `json:"id"`
	ChallengeID    string `json:"challenge_id"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	IsSample       bool   `json:"is_sample"`
	Ordinal        int    `json:"ordinal"`
}

type CreateTestCaseRequest struct {
	Input          string
	ExpectedOutput string
	IsSample       bool
}

type SampleTestCase struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	Ordinal        int    `json:"ordinal"`
}

type ChallengeDetail struct {
	Challenge
	SampleTestCases []SampleTestCase `json:"sample_test_cases"`
}

type Submission struct {
	ID          string    `json:"id"`
	ChallengeID string    `json:"challenge_id"`
	Language    string    `json:"language"`
	SourceCode  string    `json:"source_code"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateSubmissionRequest struct {
	ChallengeID string
	Language    string
	SourceCode  string
}

type Store struct {
	db  *sql.DB
	rdb *redis.Client
}

func New(db *sql.DB, rdb *redis.Client) *Store {
	return &Store{db: db, rdb: rdb}
}

const challengeColumns = `id, slug, title, description, difficulty, time_limit_ms, memory_limit_mb, created_at, updated_at`

func scanChallenge(row interface {
	Scan(...any) error
}) (Challenge, error) {
	var c Challenge
	err := row.Scan(&c.ID, &c.Slug, &c.Title, &c.Description, &c.Difficulty,
		&c.TimeLimitMs, &c.MemoryLimitMb, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *Store) ListChallenges(ctx context.Context, limit, offset int) ([]Challenge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+challengeColumns+` FROM challenges ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	challenges := make([]Challenge, 0)
	for rows.Next() {
		c, err := scanChallenge(rows)
		if err != nil {
			return nil, err
		}
		challenges = append(challenges, c)
	}
	return challenges, rows.Err()
}

func (s *Store) CreateChallenge(ctx context.Context, req CreateChallengeRequest) (Challenge, error) {
	timeLimitMs := req.TimeLimitMs
	if timeLimitMs == 0 {
		timeLimitMs = 2000
	}
	memLimitMb := req.MemoryLimitMb
	if memLimitMb == 0 {
		memLimitMb = 256
	}

	row := s.db.QueryRowContext(ctx,
		`INSERT INTO challenges (slug, title, description, difficulty, time_limit_ms, memory_limit_mb)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+challengeColumns,
		req.Slug, req.Title, req.Description, req.Difficulty, timeLimitMs, memLimitMb)

	c, err := scanChallenge(row)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return Challenge{}, ErrConflict
		}
		return Challenge{}, err
	}
	return c, nil
}

func (s *Store) GetChallenge(ctx context.Context, id string) (Challenge, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+challengeColumns+` FROM challenges WHERE id = $1`, id)
	c, err := scanChallenge(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Challenge{}, ErrNotFound
	}
	return c, err
}

func (s *Store) GetChallengeDetail(ctx context.Context, id string) (ChallengeDetail, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+challengeColumns+` FROM challenges WHERE id = $1`, id)
	c, err := scanChallenge(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ChallengeDetail{}, ErrNotFound
	}
	if err != nil {
		return ChallengeDetail{}, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT input, expected_output, ordinal FROM test_cases
		 WHERE challenge_id = $1 AND is_sample = true ORDER BY ordinal`, id)
	if err != nil {
		return ChallengeDetail{}, err
	}
	defer rows.Close()

	samples := make([]SampleTestCase, 0)
	for rows.Next() {
		var tc SampleTestCase
		if err := rows.Scan(&tc.Input, &tc.ExpectedOutput, &tc.Ordinal); err != nil {
			return ChallengeDetail{}, err
		}
		samples = append(samples, tc)
	}
	if err := rows.Err(); err != nil {
		return ChallengeDetail{}, err
	}

	return ChallengeDetail{Challenge: c, SampleTestCases: samples}, nil
}

func (s *Store) DeleteChallenge(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM test_cases WHERE challenge_id = $1`, id)
	if err != nil {
		return err
	}
	_ = res

	res, err = tx.ExecContext(ctx, `DELETE FROM challenges WHERE id = $1`, id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23503" {
			return ErrConflict
		}
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (s *Store) CreateTestCase(ctx context.Context, challengeID string, req CreateTestCaseRequest) (TestCase, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TestCase{}, err
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM challenges WHERE id = $1)`, challengeID).Scan(&exists); err != nil {
		return TestCase{}, err
	}
	if !exists {
		return TestCase{}, ErrNotFound
	}

	var tc TestCase
	err = tx.QueryRowContext(ctx,
		`INSERT INTO test_cases (challenge_id, input, expected_output, is_sample, ordinal)
		 VALUES ($1, $2, $3, $4,
		         (SELECT COALESCE(MAX(ordinal), -1) + 1 FROM test_cases WHERE challenge_id = $1))
		 RETURNING id, challenge_id, input, expected_output, is_sample, ordinal`,
		challengeID, req.Input, req.ExpectedOutput, req.IsSample,
	).Scan(&tc.ID, &tc.ChallengeID, &tc.Input, &tc.ExpectedOutput, &tc.IsSample, &tc.Ordinal)
	if err != nil {
		return TestCase{}, err
	}
	return tc, tx.Commit()
}

func (s *Store) CreateSubmission(ctx context.Context, req CreateSubmissionRequest) (Submission, error) {
	var sub Submission
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO submissions (challenge_id, language, source_code)
		 VALUES ($1, $2, $3)
		 RETURNING id, challenge_id, language, source_code, status, created_at, updated_at`,
		req.ChallengeID, req.Language, req.SourceCode,
	).Scan(&sub.ID, &sub.ChallengeID, &sub.Language, &sub.SourceCode,
		&sub.Status, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23503" {
			return Submission{}, ErrNotFound
		}
		return Submission{}, err
	}

	if s.rdb != nil {
		payload, _ := json.Marshal(map[string]string{
			"submission_id": sub.ID,
			"challenge_id":  sub.ChallengeID,
		})
		if err := s.rdb.LPush(ctx, "submissions", payload).Err(); err != nil {
			return Submission{}, fmt.Errorf("enqueue submission: %w", err)
		}
	}

	return sub, nil
}

func (s *Store) GetSubmission(ctx context.Context, id string) (Submission, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, challenge_id, language, source_code, status, created_at, updated_at
		 FROM submissions WHERE id = $1`, id)
	var sub Submission
	if err := row.Scan(&sub.ID, &sub.ChallengeID, &sub.Language, &sub.SourceCode, &sub.Status, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Submission{}, ErrNotFound
		}
		return Submission{}, err
	}
	return sub, nil
}

func (s *Store) ListTestCases(ctx context.Context, challengeID string) ([]TestCase, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM challenges WHERE id = $1)`, challengeID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, challenge_id, input, expected_output, is_sample, ordinal
		 FROM test_cases WHERE challenge_id = $1 ORDER BY ordinal ASC`, challengeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tcs := make([]TestCase, 0)
	for rows.Next() {
		var tc TestCase
		if err := rows.Scan(&tc.ID, &tc.ChallengeID, &tc.Input, &tc.ExpectedOutput, &tc.IsSample, &tc.Ordinal); err != nil {
			return nil, err
		}
		tcs = append(tcs, tc)
	}
	return tcs, rows.Err()
}
