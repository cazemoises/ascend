package store

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// SubmissionAttempt is one real (non-test-run) submission a student made for
// a challenge — metadata only, never the source code. The code itself is
// reachable via GET /submissions/:id, which the frontend links to instead of
// duplicating it here.
type SubmissionAttempt struct {
	SubmissionID string    `json:"submission_id"`
	Verdict      string    `json:"verdict"`
	CreatedAt    time.Time `json:"created_at"`
	Language     string    `json:"language"`
	ExecTimeMs   *int      `json:"exec_time_ms"`
}

// StudentChallengeSummary is every attempt a student made at one challenge,
// oldest first, plus verdict counts across those attempts.
type StudentChallengeSummary struct {
	ChallengeID       string              `json:"challenge_id"`
	ChallengeTitle    string              `json:"challenge_title"`
	Attempts          []SubmissionAttempt `json:"attempts"`
	TotalAttempts     int                 `json:"total_attempts"`
	AcceptedCount     int                 `json:"accepted_count"`
	WrongAnswerCount  int                 `json:"wrong_answer_count"`
	RuntimeErrorCount int                 `json:"runtime_error_count"`
	TimeoutCount      int                 `json:"timeout_count"`
}

// StudentStats aggregates a student's activity across every challenge
// they've attempted.
type StudentStats struct {
	TotalSubmissions    int `json:"total_submissions"`
	ChallengesAttempted int `json:"challenges_attempted"`
	ChallengesAccepted  int `json:"challenges_accepted"`
	// AvgAttemptsToSolve is the mean number of attempts up to (and including)
	// the first accepted submission, averaged only over challenges the
	// student actually solved. Nil when they haven't solved any — averaging
	// over zero challenges is undefined, not zero.
	AvgAttemptsToSolve *float64 `json:"avg_attempts_to_solve"`
}

type StudentListCompletion struct {
	ListTitle   string    `json:"list_title"`
	ItemTitle   string    `json:"item_title"`
	CompletedAt time.Time `json:"completed_at"`
}

// StudentOverview is one row of the teacher's flat student roster — no class
// grouping, since Turmas no longer exists.
type StudentOverview struct {
	StudentID   string                    `json:"student_id"`
	Email       string                    `json:"email"`
	Stats       StudentStats              `json:"stats"`
	Challenges  []StudentChallengeSummary `json:"challenges"`
	Completions []StudentListCompletion   `json:"list_completions"`
}

// TeacherStudentsOverview returns every student (role=student) who has at
// least one real (non-test-run) submission or one list item completion.
// Students with neither are omitted entirely.
func (s *Store) TeacherStudentsOverview(ctx context.Context) ([]StudentOverview, error) {
	byStudent := make(map[string]*StudentOverview)
	order := make([]string, 0)
	challengeIndex := make(map[string]map[string]int) // studentID -> challengeID -> index into Challenges

	get := func(id, email string) *StudentOverview {
		so, ok := byStudent[id]
		if !ok {
			so = &StudentOverview{
				StudentID:   id,
				Email:       email,
				Challenges:  []StudentChallengeSummary{},
				Completions: []StudentListCompletion{},
			}
			byStudent[id] = so
			order = append(order, id)
		}
		return so
	}

	attemptRows, err := s.db.QueryContext(ctx, `
		SELECT sub.user_id, u.email, sub.challenge_id, ch.title AS challenge_title,
		       sub.id, sub.status, sub.created_at, sub.language, sub.exec_time_ms
		FROM submissions sub
		JOIN challenges ch ON ch.id = sub.challenge_id
		JOIN users u ON u.id = sub.user_id AND u.role = 'student'
		WHERE sub.is_test_run = false
		ORDER BY sub.user_id, sub.challenge_id, sub.created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query student submission attempts: %w", err)
	}
	defer attemptRows.Close()
	for attemptRows.Next() {
		var studentID, email, challengeID, challengeTitle string
		var attempt SubmissionAttempt
		if err := attemptRows.Scan(&studentID, &email, &challengeID, &challengeTitle,
			&attempt.SubmissionID, &attempt.Verdict, &attempt.CreatedAt, &attempt.Language, &attempt.ExecTimeMs); err != nil {
			return nil, fmt.Errorf("scan student submission attempt: %w", err)
		}

		so := get(studentID, email)
		idxByChallenge, ok := challengeIndex[studentID]
		if !ok {
			idxByChallenge = make(map[string]int)
			challengeIndex[studentID] = idxByChallenge
		}
		idx, ok := idxByChallenge[challengeID]
		if !ok {
			so.Challenges = append(so.Challenges, StudentChallengeSummary{
				ChallengeID:    challengeID,
				ChallengeTitle: challengeTitle,
				Attempts:       []SubmissionAttempt{},
			})
			idx = len(so.Challenges) - 1
			idxByChallenge[challengeID] = idx
		}

		cs := &so.Challenges[idx]
		cs.Attempts = append(cs.Attempts, attempt)
		cs.TotalAttempts++
		switch attempt.Verdict {
		case "accepted":
			cs.AcceptedCount++
		case "wrong_answer":
			cs.WrongAnswerCount++
		case "runtime_error":
			cs.RuntimeErrorCount++
		case "time_limit_exceeded":
			cs.TimeoutCount++
		}
	}
	if err := attemptRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate student submission attempts: %w", err)
	}

	completionRows, err := s.db.QueryContext(ctx, `
		SELECT lic.student_id, u.email, pl.title AS list_title, li.title AS item_title, lic.completed_at
		FROM list_item_completions lic
		JOIN list_items li ON li.id = lic.list_item_id
		JOIN problem_lists pl ON pl.id = li.list_id
		JOIN users u ON u.id = lic.student_id AND u.role = 'student'
		ORDER BY lic.completed_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query student list completions: %w", err)
	}
	defer completionRows.Close()
	for completionRows.Next() {
		var studentID, email, listTitle, itemTitle string
		var completedAt time.Time
		if err := completionRows.Scan(&studentID, &email, &listTitle, &itemTitle, &completedAt); err != nil {
			return nil, fmt.Errorf("scan student list completion: %w", err)
		}
		so := get(studentID, email)
		so.Completions = append(so.Completions, StudentListCompletion{
			ListTitle: listTitle, ItemTitle: itemTitle, CompletedAt: completedAt,
		})
	}
	if err := completionRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate student list completions: %w", err)
	}

	overview := make([]StudentOverview, 0, len(order))
	for _, id := range order {
		so := byStudent[id]
		so.Stats = computeStudentStats(so.Challenges)
		overview = append(overview, *so)
	}
	sort.Slice(overview, func(i, j int) bool { return overview[i].Email < overview[j].Email })
	return overview, nil
}

// computeStudentStats derives the roll-up stats from a student's per-
// challenge summaries, which already carry every real attempt in
// chronological order.
func computeStudentStats(challenges []StudentChallengeSummary) StudentStats {
	stats := StudentStats{ChallengesAttempted: len(challenges)}

	var solvedAttemptsSum, solvedCount int
	for _, cs := range challenges {
		stats.TotalSubmissions += cs.TotalAttempts
		if cs.AcceptedCount == 0 {
			continue
		}
		stats.ChallengesAccepted++
		for i, attempt := range cs.Attempts {
			if attempt.Verdict == "accepted" {
				solvedAttemptsSum += i + 1
				solvedCount++
				break
			}
		}
	}
	if solvedCount > 0 {
		avg := float64(solvedAttemptsSum) / float64(solvedCount)
		stats.AvgAttemptsToSolve = &avg
	}
	return stats
}
