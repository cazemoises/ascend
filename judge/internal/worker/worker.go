package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type SubmissionJob struct {
	SubmissionID string `json:"submission_id"`
	ChallengeID  string `json:"challenge_id"`
}

type submissionRecord struct {
	ID          string
	ChallengeID string
	Language    string
	SourceCode  string
}

type challengeRecord struct {
	ID            string
	TimeLimitMs   int
	MemoryLimitMB int
	StarterCode   string
	SQLSchema     string
}

type testCaseRecord struct {
	Input          string
	ExpectedOutput string
	// OrderMatters opts a single case out of the default order-insensitive
	// comparison SQL challenges use (see outputsMatch); irrelevant for every
	// other language, which always compares exactly.
	OrderMatters bool
	// IsSample gates whether a wrong_answer verdict on this case is allowed
	// to surface its Input on the result page (see evaluate) — false for a
	// hidden case means the input must never leave the server.
	IsSample bool
}

type queue interface {
	pop(ctx context.Context) (string, error)
}

type redisQueue struct {
	client *redis.Client
	key    string
}

func (q *redisQueue) pop(ctx context.Context) (string, error) {
	res, err := q.client.BLPop(ctx, 5*time.Second, q.key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return res[1], nil
}

type Worker struct {
	queue    queue
	db       *sql.DB
	executor DockerExecutor
	logger   *slog.Logger
}

func New(client *redis.Client, db *sql.DB, executor DockerExecutor, logger *slog.Logger) *Worker {
	return &Worker{
		queue:    &redisQueue{client: client, key: "submissions"},
		db:       db,
		executor: executor,
		logger:   logger,
	}
}

func (w *Worker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		raw, err := w.queue.pop(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.Error("queue pop error", "err", err)
			continue
		}
		if raw == "" {
			continue
		}

		job, err := parseSubmissionJob(raw)
		if err != nil {
			w.logger.Error("invalid submission job payload", "payload", raw, "err", err)
			continue
		}

		if err := w.processSubmission(ctx, job); err != nil {
			w.logger.Error("submission processing failed", "submission_id", job.SubmissionID, "challenge_id", job.ChallengeID, "err", err)
		}
	}
}

func parseSubmissionJob(raw string) (SubmissionJob, error) {
	var job SubmissionJob
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		return SubmissionJob{}, fmt.Errorf("decode submission job: %w", err)
	}
	if job.SubmissionID == "" || job.ChallengeID == "" {
		return SubmissionJob{}, errors.New("submission_id and challenge_id are required")
	}
	return job, nil
}

func (w *Worker) processSubmission(ctx context.Context, job SubmissionJob) error {
	if w.db == nil {
		return errors.New("database is not configured")
	}
	if w.executor == nil {
		return errors.New("executor is not configured")
	}

	submission, err := w.fetchSubmission(ctx, job.SubmissionID)
	if err != nil {
		return fmt.Errorf("fetch submission %s: %w", job.SubmissionID, err)
	}
	if submission.ChallengeID != job.ChallengeID {
		return fmt.Errorf("submission challenge mismatch: submission=%s job=%s", submission.ChallengeID, job.ChallengeID)
	}

	challenge, err := w.fetchChallenge(ctx, job.ChallengeID)
	if err != nil {
		return fmt.Errorf("fetch challenge %s: %w", job.ChallengeID, err)
	}

	testCases, err := w.fetchTestCases(ctx, challenge.ID)
	if err != nil {
		return fmt.Errorf("fetch test cases %s: %w", challenge.ID, err)
	}

	sourceCode := buildExecutable(submission.SourceCode, challenge.StarterCode)

	res := evaluate(ctx, w.executor, submission.Language, sourceCode, challenge.SQLSchema, testCases, challenge.TimeLimitMs, challenge.MemoryLimitMB)

	if err := w.updateSubmissionResult(ctx, submission.ID, res); err != nil {
		return fmt.Errorf("update submission %s status: %w", submission.ID, err)
	}

	return nil
}

type evalResult struct {
	status      string
	passedCount int
	total       int
	execTimeMs  int
	stderr      string
	// stdout is the last-executed case's actual output — populated on
	// accepted (the last case run) and on wrong_answer (the failing case),
	// empty on runtime_error/time_limit_exceeded (stderr covers those).
	// expectedOutput is only populated alongside stdout on wrong_answer, for
	// the result page's diff display.
	stdout         string
	expectedOutput string
	// failedInput/failedIsSample are only set alongside stdout/expectedOutput
	// on wrong_answer. failedInput is deliberately left empty unless
	// failedIsSample is true — a hidden case's input must never make it
	// into the result row at all, not just be hidden by the frontend.
	failedInput    string
	failedIsSample bool
}

// evaluate runs the submission against every test case in order, stopping at
// the first failure (fail-fast), and reports how many cases passed before it
// stopped. total is len(testCases); passedCount is the number that produced
// the expected output before a wrong answer or execution error ended the run.
//
// sqlSchema is the challenge's shared DDL/base data (only meaningful when
// language is "sql"; ignored otherwise).
func evaluate(ctx context.Context, executor DockerExecutor, language, sourceCode, sqlSchema string, testCases []testCaseRecord, timeLimitMs, memoryLimitMB int) evalResult {
	res := evalResult{status: "accepted", total: len(testCases)}

	for _, testCase := range testCases {
		runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeLimitMs)*time.Millisecond)
		start := time.Now()

		runSource := sourceCode
		runInput := testCase.Input
		if Language(language) == LanguageSQL {
			// SQL challenges have no stdin harness: sqlite3 reads its entire
			// script — shared schema, this case's seed INSERTs, then the
			// student's query — from the "source file" instead, so Input is
			// deliberately left empty (there's nothing to feed on stdin).
			runSource = buildSQLScript(sqlSchema, testCase.Input, sourceCode)
			runInput = ""
		}

		result, runErr := executor.Run(runCtx, RunRequest{
			Language:      Language(language),
			SourceCode:    runSource,
			Input:         runInput,
			MemoryLimitMB: memoryLimitMB,
		})
		cancel()

		// Telemetry: the recorded execution time is the slowest test case,
		// which is the run that decides a time_limit_exceeded verdict.
		if elapsed := int(time.Since(start).Milliseconds()); elapsed > res.execTimeMs {
			res.execTimeMs = elapsed
		}

		if runErr != nil {
			res.stderr = result.Stderr
			switch {
			case errors.Is(runErr, ErrTimeLimitExceeded):
				res.status = "time_limit_exceeded"
			default:
				res.status = "runtime_error"
			}
			return res
		}

		if !outputsMatch(language, testCase.OrderMatters, result.Stdout, testCase.ExpectedOutput) {
			res.status = "wrong_answer"
			res.stdout = result.Stdout
			res.expectedOutput = testCase.ExpectedOutput
			res.failedIsSample = testCase.IsSample
			if testCase.IsSample {
				res.failedInput = testCase.Input
			}
			return res
		}

		res.passedCount++
		// Overwritten every passing case, so once the loop ends (either here,
		// falling through to accepted, or via an early return above) it holds
		// the last-executed case's actual output.
		res.stdout = result.Stdout
	}

	return res
}

// buildSQLScript assembles the script executed for one SQL test case: the
// challenge's shared DDL/base data, this case's seed INSERTs (which vary per
// case so a hardcoded answer can't pass), and finally the student's query —
// all plain SQL statements run in sequence by sqlite3.
func buildSQLScript(schema, seed, studentQuery string) string {
	return strings.Join([]string{schema, seed, studentQuery}, "\n\n")
}

// outputsMatch compares a run's stdout against a test case's expected
// output. SQL result sets are unordered by default — a query has no defined
// row order without an explicit ORDER BY — so SQL cases compare the sorted
// lines of both sides unless order_matters opts a specific case out (e.g. an
// exercise that specifically tests correct ORDER BY usage). Every other
// language, and SQL cases with order_matters, compare exactly.
func outputsMatch(language string, orderMatters bool, actual, expected string) bool {
	if Language(language) == LanguageSQL && !orderMatters {
		return sortedLines(actual) == sortedLines(expected)
	}
	return normalizeOutput(actual) == normalizeOutput(expected)
}

func sortedLines(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func (w *Worker) fetchSubmission(ctx context.Context, id string) (submissionRecord, error) {
	row := w.db.QueryRowContext(ctx,
		`SELECT id, challenge_id, language, source_code FROM submissions WHERE id = $1`, id)

	var submission submissionRecord
	if err := row.Scan(&submission.ID, &submission.ChallengeID, &submission.Language, &submission.SourceCode); err != nil {
		return submissionRecord{}, fmt.Errorf("scan submission %s: %w", id, err)
	}
	return submission, nil
}

func (w *Worker) fetchChallenge(ctx context.Context, id string) (challengeRecord, error) {
	row := w.db.QueryRowContext(ctx,
		`SELECT time_limit_ms, memory_limit_mb, COALESCE(starter_code, ''), COALESCE(sql_schema, '')
		 FROM challenges WHERE id = $1`, id)

	var challenge challengeRecord
	challenge.ID = id
	if err := row.Scan(&challenge.TimeLimitMs, &challenge.MemoryLimitMB, &challenge.StarterCode, &challenge.SQLSchema); err != nil {
		return challengeRecord{}, fmt.Errorf("scan challenge %s: %w", id, err)
	}
	return challenge, nil
}

// runnerMarker splits a challenge's starter_code in two: the student-visible
// stub above the marker line and the teacher's hidden stdin/stdout harness
// below it. The frontend shows only the stub; at execution time the student's
// submission is concatenated above the harness into one runtime script.
const runnerMarker = "[[ASCEND::RUNNER]]"

// buildExecutable assembles the script handed to the sandbox. Challenges
// without a marker (or without starter_code at all) run the raw submission
// unchanged, which keeps every pre-marker challenge working as before.
func buildExecutable(studentCode, starterCode string) string {
	idx := strings.Index(starterCode, runnerMarker)
	if idx == -1 {
		return studentCode
	}
	rest := starterCode[idx:]
	nl := strings.IndexByte(rest, '\n')
	if nl == -1 {
		// Marker is the last line: there is no harness underneath it.
		return studentCode
	}
	harness := rest[nl+1:]
	return strings.TrimRight(studentCode, "\n") + "\n\n" + harness
}

func (w *Worker) fetchTestCases(ctx context.Context, challengeID string) ([]testCaseRecord, error) {
	rows, err := w.db.QueryContext(ctx,
		`SELECT input, expected_output, order_matters, is_sample FROM test_cases WHERE challenge_id = $1 ORDER BY ordinal`, challengeID)
	if err != nil {
		return nil, fmt.Errorf("query test cases %s: %w", challengeID, err)
	}
	defer rows.Close()

	var testCases []testCaseRecord
	for rows.Next() {
		var testCase testCaseRecord
		if err := rows.Scan(&testCase.Input, &testCase.ExpectedOutput, &testCase.OrderMatters, &testCase.IsSample); err != nil {
			return nil, fmt.Errorf("scan test case %s: %w", challengeID, err)
		}
		testCases = append(testCases, testCase)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate test cases %s: %w", challengeID, err)
	}

	return testCases, nil
}

// maxStderrBytes bounds the stored crash log so a submission cannot bloat
// the row by spamming stderr.
const maxStderrBytes = 64 * 1024

// maxOutputBytes bounds the stored stdout/expected-output snapshot shown on
// a wrong-answer result — smaller than maxStderrBytes since this is a
// side-by-side diff display, not a scrollable crash log.
const maxOutputBytes = 8000

// truncate caps s at max bytes, leaving it unchanged if it's already
// shorter — used for every large field persisted onto a submission row so
// none of them can bloat it.
func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

func (w *Worker) updateSubmissionResult(ctx context.Context, submissionID string, res evalResult) error {
	stderrLog := truncate(res.stderr, maxStderrBytes)
	stdoutLog := truncate(res.stdout, maxOutputBytes)
	expectedLog := truncate(res.expectedOutput, maxOutputBytes)

	// sql.NullString/NullBool, not NULLIF(x, ''): failedInput can
	// legitimately BE an empty string for a sample case whose test input is
	// blank (e.g. a no-stdin program) — NULLIF would collapse that to NULL,
	// indistinguishable from "hidden case, deliberately withheld". Valid
	// tracks that distinction explicitly instead of overloading emptiness.
	var failedInput sql.NullString
	var failedIsSample sql.NullBool
	if res.status == "wrong_answer" {
		failedIsSample = sql.NullBool{Bool: res.failedIsSample, Valid: true}
		if res.failedIsSample {
			failedInput = sql.NullString{String: truncate(res.failedInput, maxOutputBytes), Valid: true}
		}
	}

	_, err := w.db.ExecContext(ctx,
		`UPDATE submissions
		 SET status = $1, passed_count = $2, total_test_cases = $3,
		     exec_time_ms = $4, stderr = NULLIF($5, ''),
		     stdout = NULLIF($6, ''), expected_output = NULLIF($7, ''),
		     failed_input = $8, failed_is_sample = $9,
		     updated_at = NOW()
		 WHERE id = $10`,
		res.status, res.passedCount, res.total, res.execTimeMs, stderrLog, stdoutLog, expectedLog,
		failedInput, failedIsSample, submissionID)
	if err != nil {
		return fmt.Errorf("exec submission update %s: %w", submissionID, err)
	}

	return nil
}

func normalizeOutput(output string) string {
	return strings.TrimSpace(output)
}
