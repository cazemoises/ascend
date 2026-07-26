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
	ClassID       *string   `json:"class_id"`
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
	Notes         *string
	StarterCode   *string
	ClassID       *string
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

const challengeColumns = `id, slug, title, description, difficulty, time_limit_ms, memory_limit_mb, notes, starter_code, class_id, created_at, updated_at`

func scanChallenge(row interface {
	Scan(...any) error
}) (Challenge, error) {
	var c Challenge
	err := row.Scan(&c.ID, &c.Slug, &c.Title, &c.Description, &c.Difficulty,
		&c.TimeLimitMs, &c.MemoryLimitMb, &c.Notes, &c.StarterCode, &c.ClassID, &c.CreatedAt, &c.UpdatedAt)
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

// ListChallengesForViewer applies class visibility: anonymous viewers get
// only public challenges (class_id IS NULL); students additionally get
// challenges published to classes they are enrolled in; teachers see all.
func (s *Store) ListChallengesForViewer(ctx context.Context, viewerID, role string, limit, offset int) ([]Challenge, error) {
	query := `SELECT ` + challengeColumns + ` FROM challenges
	 WHERE class_id IS NULL ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	args := []any{limit, offset}
	switch {
	case role == "teacher":
		query = `SELECT ` + challengeColumns + ` FROM challenges ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	case viewerID != "":
		query = `SELECT ` + challengeColumns + ` FROM challenges
		 WHERE class_id IS NULL
		    OR class_id IN (SELECT class_id FROM class_enrollments WHERE student_id = $3)
		 ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		args = append(args, viewerID)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectChallenges(rows)
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
		`INSERT INTO challenges (slug, title, description, difficulty, time_limit_ms, memory_limit_mb, notes, starter_code, class_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING `+challengeColumns,
		req.Slug, req.Title, req.Description, req.Difficulty, timeLimitMs, memLimitMb, req.Notes, req.StarterCode, req.ClassID)

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
// starter_code is always written so teachers can clear it.
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
		     notes = COALESCE($8, notes), starter_code = $9, class_id = $10, updated_at = now()
		 WHERE id = $1
		 RETURNING `+challengeColumns,
		id, req.Slug, req.Title, req.Description, req.Difficulty,
		timeLimitMs, memLimitMb, req.Notes, req.StarterCode, req.ClassID)

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
			`INSERT INTO test_cases (challenge_id, input, expected_output, is_sample, ordinal)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING id, challenge_id, input, expected_output, is_sample, ordinal`,
			challengeID, req.Input, req.ExpectedOutput, req.IsSample, i,
		).Scan(&tc.ID, &tc.ChallengeID, &tc.Input, &tc.ExpectedOutput, &tc.IsSample, &tc.Ordinal)
		if err != nil {
			return nil, err
		}
		tcs = append(tcs, tc)
	}
	return tcs, tx.Commit()
}

func (s *Store) CreateSubmission(ctx context.Context, req CreateSubmissionRequest) (Submission, error) {
	var sub Submission
	err := s.db.QueryRowContext(ctx,
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

// ClassScore is one row of the teacher scoreboard: a student's completion
// count against the total challenges published to that class.
type ClassScore struct {
	ClassID      string `json:"class_id"`
	ClassName    string `json:"class_name"`
	StudentID    string `json:"student_id"`
	StudentEmail string `json:"student_email"`
	Completed    int    `json:"completed"`
	Total        int    `json:"total"`
}

func (s *Store) TeacherScoreboard(ctx context.Context, teacherID string) ([]ClassScore, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT cl.id, cl.name, u.id, u.email,
		        (SELECT COUNT(DISTINCT sub.challenge_id) FROM submissions sub
		           JOIN challenges ch ON ch.id = sub.challenge_id AND ch.class_id = cl.id
		           WHERE sub.user_id = u.id AND sub.status = 'accepted'
		             AND sub.is_test_run = false) AS completed,
		        (SELECT COUNT(*) FROM challenges ch WHERE ch.class_id = cl.id) AS total
		 FROM classes cl
		 JOIN class_enrollments ce ON ce.class_id = cl.id
		 JOIN users u ON u.id = ce.student_id
		 WHERE cl.teacher_id = $1
		 ORDER BY cl.name, u.email`, teacherID)
	if err != nil {
		return nil, fmt.Errorf("query teacher scoreboard: %w", err)
	}
	defer rows.Close()

	// Always non-nil so an empty scoreboard serializes as [] rather than null.
	scores := make([]ClassScore, 0)
	for rows.Next() {
		var sc ClassScore
		var completed, total sql.NullInt64
		if err := rows.Scan(&sc.ClassID, &sc.ClassName, &sc.StudentID, &sc.StudentEmail, &completed, &total); err != nil {
			return nil, fmt.Errorf("scan teacher scoreboard row: %w", err)
		}
		sc.Completed = int(completed.Int64)
		sc.Total = int(total.Int64)
		scores = append(scores, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate teacher scoreboard: %w", err)
	}
	return scores, nil
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
