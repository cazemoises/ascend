package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
}

type testCaseRecord struct {
	Input          string
	ExpectedOutput string
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

	res := evaluate(ctx, w.executor, submission.Language, sourceCode, testCases, challenge.TimeLimitMs, challenge.MemoryLimitMB)

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
}

// evaluate runs the submission against every test case in order, stopping at
// the first failure (fail-fast), and reports how many cases passed before it
// stopped. total is len(testCases); passedCount is the number that produced
// the expected output before a wrong answer or execution error ended the run.
func evaluate(ctx context.Context, executor DockerExecutor, language, sourceCode string, testCases []testCaseRecord, timeLimitMs, memoryLimitMB int) evalResult {
	res := evalResult{status: "accepted", total: len(testCases)}

	for _, testCase := range testCases {
		runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeLimitMs)*time.Millisecond)
		start := time.Now()
		result, runErr := executor.Run(runCtx, RunRequest{
			Language:      Language(language),
			SourceCode:    sourceCode,
			Input:         testCase.Input,
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

		if normalizeOutput(result.Stdout) != normalizeOutput(testCase.ExpectedOutput) {
			res.status = "wrong_answer"
			return res
		}

		res.passedCount++
	}

	return res
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
		`SELECT time_limit_ms, memory_limit_mb, COALESCE(starter_code, '') FROM challenges WHERE id = $1`, id)

	var challenge challengeRecord
	challenge.ID = id
	if err := row.Scan(&challenge.TimeLimitMs, &challenge.MemoryLimitMB, &challenge.StarterCode); err != nil {
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
		`SELECT input, expected_output FROM test_cases WHERE challenge_id = $1 ORDER BY ordinal`, challengeID)
	if err != nil {
		return nil, fmt.Errorf("query test cases %s: %w", challengeID, err)
	}
	defer rows.Close()

	var testCases []testCaseRecord
	for rows.Next() {
		var testCase testCaseRecord
		if err := rows.Scan(&testCase.Input, &testCase.ExpectedOutput); err != nil {
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

func (w *Worker) updateSubmissionResult(ctx context.Context, submissionID string, res evalResult) error {
	stderrLog := res.stderr
	if len(stderrLog) > maxStderrBytes {
		stderrLog = stderrLog[:maxStderrBytes]
	}
	_, err := w.db.ExecContext(ctx,
		`UPDATE submissions
		 SET status = $1, passed_count = $2, total_test_cases = $3,
		     exec_time_ms = $4, stderr = NULLIF($5, ''), updated_at = NOW()
		 WHERE id = $6`,
		res.status, res.passedCount, res.total, res.execTimeMs, stderrLog, submissionID)
	if err != nil {
		return fmt.Errorf("exec submission update %s: %w", submissionID, err)
	}

	return nil
}

func normalizeOutput(output string) string {
	return strings.TrimSpace(output)
}
