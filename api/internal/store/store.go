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
	Notes         *string   `json:"notes"`
	StarterCode   *string   `json:"starter_code"`
	// Language is nil for today's multi-language challenges (the student
	// picks python/go/javascript per submission) or "sql" for a SQL-only
	// challenge, which uses SQLSchema instead of StarterCode.
	Language  *string   `json:"language"`
	SQLSchema *string   `json:"sql_schema"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateChallengeRequest struct {
	Slug          string
	Title         string
	Description   string
	Difficulty    string
	TimeLimitMs   int
	MemoryLimitMb int
	Notes         *string
	StarterCode   *string
	Language      *string
	SQLSchema     *string
}

type TestCase struct {
	ID             string `json:"id"`
	ChallengeID    string `json:"challenge_id"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	IsSample       bool   `json:"is_sample"`
	Ordinal        int    `json:"ordinal"`
	// OrderMatters opts this case out of the default order-insensitive
	// comparison SQL challenges use; meaningless for every other language.
	OrderMatters bool `json:"order_matters"`
}

type CreateTestCaseRequest struct {
	Input          string
	ExpectedOutput string
	IsSample       bool
	OrderMatters   bool
}

type SampleTestCase struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	Ordinal        int    `json:"ordinal"`
}

// ChallengeDetail is a Challenge annotated with whether the requesting
// viewer has already solved it — same rule as ChallengeFeedItem.Solved,
// just for the single-challenge detail endpoint.
type ChallengeDetail struct {
	Challenge
	SampleTestCases []SampleTestCase `json:"sample_test_cases"`
	Solved          bool             `json:"solved"`
}

// ChallengeFeedItem is a Challenge annotated with whether the requesting
// viewer has already solved it — only ListChallengesForViewer populates
// Solved; every other Challenge-returning query leaves it at its zero value.
type ChallengeFeedItem struct {
	Challenge
	Solved bool `json:"solved"`
}

type Submission struct {
	ID           string  `json:"id"`
	ChallengeID  string  `json:"challenge_id"`
	Language     string  `json:"language"`
	SourceCode   string  `json:"source_code"`
	Status       string  `json:"status"`
	ExecTimeMs   *int    `json:"exec_time_ms"`
	MemoryPeakMb *int    `json:"memory_peak_mb"`
	Stderr       *string `json:"stderr"`
	// Nullable "X of Y test cases passed"; NULL for submissions judged before
	// the counts existed, or that failed before the worker ran a single case.
	PassedCount    *int `json:"passed_count"`
	TotalTestCases *int `json:"total_test_cases"`
	// Challenge budgets, joined in for "120ms / 2000ms" style displays.
	TimeLimitMs   int       `json:"time_limit_ms"`
	MemoryLimitMb int       `json:"memory_limit_mb"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateSubmissionRequest struct {
	ChallengeID string
	UserID      string
	Language    string
	SourceCode  string
	// IsTestRun is derived server-side from the caller's JWT role — never
	// trust this from client input.
	IsTestRun bool
}

type Store struct {
	db  *sql.DB
	rdb *redis.Client
}

func New(db *sql.DB, rdb *redis.Client) *Store {
	return &Store{db: db, rdb: rdb}
}

const challengeColumns = `id, slug, title, description, difficulty, time_limit_ms, memory_limit_mb, notes, starter_code, language, sql_schema, created_at, updated_at`

func scanChallenge(row interface {
	Scan(...any) error
}) (Challenge, error) {
	var c Challenge
	err := row.Scan(&c.ID, &c.Slug, &c.Title, &c.Description, &c.Difficulty,
		&c.TimeLimitMs, &c.MemoryLimitMb, &c.Notes, &c.StarterCode, &c.Language, &c.SQLSchema,
		&c.CreatedAt, &c.UpdatedAt)
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

	return collectChallenges(rows)
}

// challengeColumnsAliased is challengeColumns qualified with the "c" alias
// ListChallengesForViewer joins under, so it can sit alongside the solved
// subquery below in the same SELECT list.
const challengeColumnsAliased = `c.id, c.slug, c.title, c.description, c.difficulty, c.time_limit_ms, c.memory_limit_mb, c.notes, c.starter_code, c.language, c.sql_schema, c.created_at, c.updated_at`

// solvedExpr is the raw EXISTS(...) boolean expression reporting whether the
// viewer (positional parameter paramIndex, '' for anonymous) has an
// accepted, non-test-run submission for the challenge identified by
// challengeIDExpr (e.g. "c.id", or "li.linked_challenge_id" for a list item
// linked to a challenge) — the one place this rule is defined, reused by
// every query that needs it instead of a parallel copy. NULLIF('', '')::uuid
// turns an empty viewerID into NULL, which never equals any user_id — so
// anonymous viewers naturally always get false.
func solvedExpr(challengeIDExpr string, paramIndex int) string {
	return fmt.Sprintf(`EXISTS(
		SELECT 1 FROM submissions sub
		WHERE sub.challenge_id = %s AND sub.user_id = NULLIF($%d, '')::uuid
		  AND sub.status = 'accepted' AND sub.is_test_run = false
	)`, challengeIDExpr, paramIndex)
}

// solvedColumn is solvedExpr aliased as "solved", for queries that select it
// directly as a column (as opposed to embedding it inside a larger
// expression, like the list-item completion CASE in lists.go).
func solvedColumn(challengeIDExpr string, paramIndex int) string {
	return solvedExpr(challengeIDExpr, paramIndex) + " AS solved"
}

func scanChallengeFeedItem(row interface {
	Scan(...any) error
}) (ChallengeFeedItem, error) {
	var item ChallengeFeedItem
	err := row.Scan(&item.ID, &item.Slug, &item.Title, &item.Description, &item.Difficulty,
		&item.TimeLimitMs, &item.MemoryLimitMb, &item.Notes, &item.StarterCode, &item.Language, &item.SQLSchema,
		&item.CreatedAt, &item.UpdatedAt, &item.Solved)
	return item, err
}

// ListChallengesForViewer serves every challenge, ordered newest-first, with
// each item annotated with whether viewerID has already solved it.
func (s *Store) ListChallengesForViewer(ctx context.Context, viewerID string, limit, offset int) ([]ChallengeFeedItem, error) {
	query := `SELECT ` + challengeColumnsAliased + `, ` + solvedColumn("c.id", 3) + ` FROM challenges c
	 ORDER BY c.created_at DESC LIMIT $1 OFFSET $2`
	args := []any{limit, offset, viewerID}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ChallengeFeedItem, 0)
	for rows.Next() {
		item, err := scanChallengeFeedItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func collectChallenges(rows *sql.Rows) ([]Challenge, error) {

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
		`INSERT INTO challenges (slug, title, description, difficulty, time_limit_ms, memory_limit_mb, notes, starter_code, language, sql_schema)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING `+challengeColumns,
		req.Slug, req.Title, req.Description, req.Difficulty, timeLimitMs, memLimitMb, req.Notes, req.StarterCode,
		req.Language, req.SQLSchema)

	c, err := scanChallenge(row)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return Challenge{}, ErrConflict
		}
		return Challenge{}, err
	}
	return c, nil
}

// UpdateChallenge overwrites the editable fields of a challenge. Notes are
// preserved when the request carries nil (the studio has no notes field);
// starter_code, language, and sql_schema are always written so teachers can
// clear them.
func (s *Store) UpdateChallenge(ctx context.Context, id string, req CreateChallengeRequest) (Challenge, error) {
	timeLimitMs := req.TimeLimitMs
	if timeLimitMs == 0 {
		timeLimitMs = 2000
	}
	memLimitMb := req.MemoryLimitMb
	if memLimitMb == 0 {
		memLimitMb = 256
	}

	row := s.db.QueryRowContext(ctx,
		`UPDATE challenges
		 SET slug = $2, title = $3, description = $4, difficulty = $5,
		     time_limit_ms = $6, memory_limit_mb = $7,
		     notes = COALESCE($8, notes), starter_code = $9,
		     language = $10, sql_schema = $11, updated_at = now()
		 WHERE id = $1
		 RETURNING `+challengeColumns,
		id, req.Slug, req.Title, req.Description, req.Difficulty,
		timeLimitMs, memLimitMb, req.Notes, req.StarterCode, req.Language, req.SQLSchema)

	c, err := scanChallenge(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Challenge{}, ErrNotFound
		}
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

// GetChallengeDetail fetches a single challenge plus its sample test cases,
// annotated with whether viewerID ('' for anonymous) has already solved it —
// same rule and same solvedColumn helper as ListChallengesForViewer.
func (s *Store) GetChallengeDetail(ctx context.Context, id, viewerID string) (ChallengeDetail, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+challengeColumnsAliased+`, `+solvedColumn("c.id", 2)+` FROM challenges c WHERE c.id = $1`,
		id, viewerID)

	var d ChallengeDetail
	err := row.Scan(&d.ID, &d.Slug, &d.Title, &d.Description, &d.Difficulty,
		&d.TimeLimitMs, &d.MemoryLimitMb, &d.Notes, &d.StarterCode, &d.Language, &d.SQLSchema,
		&d.CreatedAt, &d.UpdatedAt, &d.Solved)
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

	d.SampleTestCases = samples
	return d, nil
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
		`INSERT INTO test_cases (challenge_id, input, expected_output, is_sample, order_matters, ordinal)
		 VALUES ($1, $2, $3, $4, $5,
		         (SELECT COALESCE(MAX(ordinal), -1) + 1 FROM test_cases WHERE challenge_id = $1))
		 RETURNING id, challenge_id, input, expected_output, is_sample, ordinal, order_matters`,
		challengeID, req.Input, req.ExpectedOutput, req.IsSample, req.OrderMatters,
	).Scan(&tc.ID, &tc.ChallengeID, &tc.Input, &tc.ExpectedOutput, &tc.IsSample, &tc.Ordinal, &tc.OrderMatters)
	if err != nil {
		return TestCase{}, err
	}
	return tc, tx.Commit()
}

// ReplaceTestCases swaps the full test suite of a challenge atomically:
// existing cases are deleted and the new ones inserted with sequential
// ordinals inside one transaction.
func (s *Store) ReplaceTestCases(ctx context.Context, challengeID string, reqs []CreateTestCaseRequest) ([]TestCase, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM challenges WHERE id = $1)`, challengeID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM test_cases WHERE challenge_id = $1`, challengeID); err != nil {
		return nil, err
	}

	tcs := make([]TestCase, 0, len(reqs))
	for i, req := range reqs {
		var tc TestCase
		err := tx.QueryRowContext(ctx,
			`INSERT INTO test_cases (challenge_id, input, expected_output, is_sample, order_matters, ordinal)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 RETURNING id, challenge_id, input, expected_output, is_sample, ordinal, order_matters`,
			challengeID, req.Input, req.ExpectedOutput, req.IsSample, req.OrderMatters, i,
		).Scan(&tc.ID, &tc.ChallengeID, &tc.Input, &tc.ExpectedOutput, &tc.IsSample, &tc.Ordinal, &tc.OrderMatters)
		if err != nil {
			return nil, err
		}
		tcs = append(tcs, tc)
	}
	return tcs, tx.Commit()
}

// ErrLanguageMismatch means the submission's language doesn't match what the
// challenge accepts: a SQL-only challenge (language='sql') requires a 'sql'
// submission, and a multi-language challenge (language IS NULL) can't take a
// 'sql' one, since only SQL-only challenges carry the sql_schema a SQL
// submission needs to run against.
var ErrLanguageMismatch = errors.New("language mismatch")

func languageAllowed(challengeLanguage sql.NullString, submissionLanguage string) bool {
	if challengeLanguage.Valid {
		return submissionLanguage == challengeLanguage.String
	}
	return submissionLanguage != "sql"
}

func (s *Store) CreateSubmission(ctx context.Context, req CreateSubmissionRequest) (Submission, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Submission{}, err
	}
	defer tx.Rollback()

	var challengeLanguage sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT language FROM challenges WHERE id = $1`, req.ChallengeID,
	).Scan(&challengeLanguage); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Submission{}, ErrNotFound
		}
		return Submission{}, err
	}
	if !languageAllowed(challengeLanguage, req.Language) {
		return Submission{}, ErrLanguageMismatch
	}

	var sub Submission
	err = tx.QueryRowContext(ctx,
		`INSERT INTO submissions (challenge_id, user_id, language, source_code, is_test_run)
		 VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5)
		 RETURNING id, challenge_id, language, source_code, status, created_at, updated_at`,
		req.ChallengeID, req.UserID, req.Language, req.SourceCode, req.IsTestRun,
	).Scan(&sub.ID, &sub.ChallengeID, &sub.Language, &sub.SourceCode,
		&sub.Status, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23503" {
			return Submission{}, ErrNotFound
		}
		return Submission{}, err
	}

	if err := tx.Commit(); err != nil {
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
		`SELECT s.id, s.challenge_id, s.language, s.source_code, s.status,
		        s.exec_time_ms, s.memory_peak_mb, s.stderr,
		        s.passed_count, s.total_test_cases,
		        c.time_limit_ms, c.memory_limit_mb,
		        s.created_at, s.updated_at
		 FROM submissions s
		 JOIN challenges c ON c.id = s.challenge_id
		 WHERE s.id = $1`, id)
	var sub Submission
	if err := row.Scan(&sub.ID, &sub.ChallengeID, &sub.Language, &sub.SourceCode, &sub.Status,
		&sub.ExecTimeMs, &sub.MemoryPeakMb, &sub.Stderr,
		&sub.PassedCount, &sub.TotalTestCases,
		&sub.TimeLimitMs, &sub.MemoryLimitMb,
		&sub.CreatedAt, &sub.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Submission{}, ErrNotFound
		}
		return Submission{}, err
	}
	return sub, nil
}

// ChallengeStats aggregates accepted-run timing for one challenge, plus the
// requesting user's standing inside that distribution.
type ChallengeStats struct {
	TotalRuns     int           `json:"total_runs"`
	UserBestMs    *int          `json:"user_best_ms"`
	FasterThanPct *int          `json:"faster_than_pct"`
	Buckets       []StatsBucket `json:"buckets"`
}

type StatsBucket struct {
	UpToMs int `json:"up_to_ms"`
	Count  int `json:"count"`
}

const statsBucketCount = 8

func (s *Store) GetChallengeStats(ctx context.Context, challengeID, userID string) (ChallengeStats, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM challenges WHERE id = $1)`, challengeID).Scan(&exists); err != nil {
		return ChallengeStats{}, err
	}
	if !exists {
		return ChallengeStats{}, ErrNotFound
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT exec_time_ms FROM submissions
		 WHERE challenge_id = $1 AND status = 'accepted' AND exec_time_ms IS NOT NULL
		   AND is_test_run = false
		 ORDER BY exec_time_ms ASC
		 LIMIT 5000`, challengeID)
	if err != nil {
		return ChallengeStats{}, fmt.Errorf("query challenge stats: %w", err)
	}
	defer rows.Close()

	times := make([]int, 0)
	for rows.Next() {
		var t int
		if err := rows.Scan(&t); err != nil {
			return ChallengeStats{}, err
		}
		times = append(times, t)
	}
	if err := rows.Err(); err != nil {
		return ChallengeStats{}, err
	}

	stats := ChallengeStats{TotalRuns: len(times), Buckets: make([]StatsBucket, 0, statsBucketCount)}
	if len(times) > 0 {
		maxMs := times[len(times)-1]
		width := maxMs/statsBucketCount + 1
		for i := 0; i < statsBucketCount; i++ {
			stats.Buckets = append(stats.Buckets, StatsBucket{UpToMs: width * (i + 1)})
		}
		for _, t := range times {
			idx := t / width
			if idx >= statsBucketCount {
				idx = statsBucketCount - 1
			}
			stats.Buckets[idx].Count++
		}
	}

	var best sql.NullInt64
	if err := s.db.QueryRowContext(ctx,
		`SELECT MIN(exec_time_ms) FROM submissions
		 WHERE challenge_id = $1 AND user_id = $2 AND status = 'accepted' AND exec_time_ms IS NOT NULL
		   AND is_test_run = false`,
		challengeID, userID).Scan(&best); err != nil {
		return ChallengeStats{}, err
	}
	if best.Valid && len(times) > 0 {
		b := int(best.Int64)
		slower := 0
		for _, t := range times {
			if t > b {
				slower++
			}
		}
		pct := slower * 100 / len(times)
		stats.UserBestMs = &b
		stats.FasterThanPct = &pct
	}

	return stats, nil
}

type SubmissionSummary struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Language  string    `json:"language"`
	IsTestRun bool      `json:"is_test_run"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) ListRecentSubmissions(ctx context.Context, challengeID string, limit int) ([]SubmissionSummary, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM challenges WHERE id = $1)`, challengeID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, status, language, is_test_run, created_at FROM submissions
		 WHERE challenge_id = $1 ORDER BY created_at DESC LIMIT $2`,
		challengeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := make([]SubmissionSummary, 0)
	for rows.Next() {
		var ss SubmissionSummary
		if err := rows.Scan(&ss.ID, &ss.Status, &ss.Language, &ss.IsTestRun, &ss.CreatedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, ss)
	}
	return summaries, rows.Err()
}

type UserSubmission struct {
	ID             string    `json:"id"`
	ChallengeID    string    `json:"challenge_id"`
	ChallengeTitle string    `json:"challenge_title"`
	Language       string    `json:"language"`
	Status         string    `json:"status"`
	IsTestRun      bool      `json:"is_test_run"`
	CreatedAt      time.Time `json:"created_at"`
}

// ListUserSubmissions returns the newest submissions for a user using keyset
// pagination: pass the created_at and id of the last row from the previous
// page (both nil/empty for the first page).
func (s *Store) ListUserSubmissions(ctx context.Context, userID string, limit int, beforeCreatedAt *time.Time, beforeID *string) ([]UserSubmission, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT s.id, s.challenge_id, c.title, s.language, s.status, s.is_test_run, s.created_at
		 FROM submissions s
		 JOIN challenges c ON c.id = s.challenge_id
		 WHERE s.user_id = $1
		   AND ($2::timestamptz IS NULL OR $3::uuid IS NULL
		        OR (s.created_at, s.id) < ($2::timestamptz, $3::uuid))
		 ORDER BY s.created_at DESC, s.id DESC
		 LIMIT $4`,
		userID, beforeCreatedAt, beforeID, limit)
	if err != nil {
		return nil, fmt.Errorf("list user submissions: %w", err)
	}
	defer rows.Close()

	subs := make([]UserSubmission, 0)
	for rows.Next() {
		var us UserSubmission
		if err := rows.Scan(&us.ID, &us.ChallengeID, &us.ChallengeTitle, &us.Language, &us.Status, &us.IsTestRun, &us.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user submission: %w", err)
		}
		subs = append(subs, us)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user submissions: %w", err)
	}
	return subs, nil
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
		`SELECT id, challenge_id, input, expected_output, is_sample, ordinal, order_matters
		 FROM test_cases WHERE challenge_id = $1 ORDER BY ordinal ASC`, challengeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tcs := make([]TestCase, 0)
	for rows.Next() {
		var tc TestCase
		if err := rows.Scan(&tc.ID, &tc.ChallengeID, &tc.Input, &tc.ExpectedOutput, &tc.IsSample, &tc.Ordinal, &tc.OrderMatters); err != nil {
			return nil, err
		}
		tcs = append(tcs, tc)
	}
	return tcs, rows.Err()
}
