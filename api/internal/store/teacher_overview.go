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
	AvgAttemptsToSolve    *float64 `json:"avg_attempts_to_solve"`
	UnresolvedChallenges  int      `json:"unresolved_challenges"`
	RecurringDifficulties int      `json:"recurring_difficulties"`
	WrongAnswerCount      int      `json:"wrong_answer_count"`
	RuntimeErrorCount     int      `json:"runtime_error_count"`
	TimeoutCount          int      `json:"timeout_count"`
}

// StudentWeek groups activity by Monday in UTC. NewlySolved counts only the
// first accepted attempt for each challenge, never repeated accepted attempts.
type StudentWeek struct {
	WeekStart   string `json:"week_start"`
	Attempts    int    `json:"attempts"`
	NewlySolved int    `json:"newly_solved"`
}

type AssessmentStats struct {
	Attempts              int `json:"attempts"`
	AcceptedAttempts      int `json:"accepted_attempts"`
	ItemsSolved           int `json:"items_solved"`
	RecurringDifficulties int `json:"recurring_difficulties"`
}

type StudentListCompletion struct {
	ListTitle   string    `json:"list_title"`
	ItemTitle   string    `json:"item_title"`
	CompletedAt time.Time `json:"completed_at"`
}

// StudentOverview is one row of the teacher's flat student roster — no class
// grouping, since Turmas no longer exists.
type StudentOverview struct {
	AssessmentStats AssessmentStats           `json:"assessment_stats"`
	StudentID       string                    `json:"student_id"`
	Email           string                    `json:"email"`
	Stats           StudentStats              `json:"stats"`
	WeeklyProgress  []StudentWeek             `json:"weekly_progress"`
	Challenges      []StudentChallengeSummary `json:"challenges"`
	Completions     []StudentListCompletion   `json:"list_completions"`
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
		ORDER BY sub.user_id, sub.challenge_id, sub.created_at ASC, sub.id ASC`)
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
        WHERE NOT EXISTS(SELECT 1 FROM list_assessment_revisions ar WHERE ar.list_item_id=li.id)
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

	assessmentRows, err := s.db.QueryContext(ctx, `SELECT a.student_id,u.email,ar.list_item_id,count(*),count(*) FILTER(WHERE a.accepted),
 count(*) FILTER(WHERE a.accepted AND ar.id=(SELECT latest.id FROM list_assessment_revisions latest WHERE latest.list_item_id=ar.list_item_id ORDER BY latest.revision DESC LIMIT 1)),
 count(*) FILTER(WHERE NOT a.accepted AND ar.id=(SELECT latest.id FROM list_assessment_revisions latest WHERE latest.list_item_id=ar.list_item_id ORDER BY latest.revision DESC LIMIT 1))
 FROM list_assessment_attempts a JOIN list_assessment_revisions ar ON ar.id=a.revision_id JOIN users u ON u.id=a.student_id AND u.role='student'
 GROUP BY a.student_id,u.email,ar.list_item_id`)
	if err != nil {
		return nil, fmt.Errorf("query assessment overview: %w", err)
	}
	defer assessmentRows.Close()
	for assessmentRows.Next() {
		var studentID, email, itemID string
		var attempts, accepted, currentAccepted, currentFailures int
		if err := assessmentRows.Scan(&studentID, &email, &itemID, &attempts, &accepted, &currentAccepted, &currentFailures); err != nil {
			return nil, fmt.Errorf("scan assessment overview: %w", err)
		}
		stats := &get(studentID, email).AssessmentStats
		stats.Attempts += attempts
		stats.AcceptedAttempts += accepted
		if currentAccepted > 0 {
			stats.ItemsSolved++
		} else if currentFailures >= 3 {
			stats.RecurringDifficulties++
		}
	}
	if err := assessmentRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assessment overview: %w", err)
	}
	assessmentRows.Close()
	gradedCompletions, err := s.db.QueryContext(ctx, `SELECT a.student_id,u.email,pl.title,li.title,min(a.created_at)
 FROM list_assessment_attempts a JOIN list_assessment_revisions ar ON ar.id=a.revision_id JOIN list_items li ON li.id=ar.list_item_id JOIN problem_lists pl ON pl.id=li.list_id JOIN users u ON u.id=a.student_id AND u.role='student'
 WHERE a.accepted AND ar.id=(SELECT latest.id FROM list_assessment_revisions latest WHERE latest.list_item_id=li.id ORDER BY latest.revision DESC LIMIT 1)
 GROUP BY a.student_id,u.email,li.id,pl.id`)
	if err != nil {
		return nil, fmt.Errorf("query graded completions: %w", err)
	}
	defer gradedCompletions.Close()
	for gradedCompletions.Next() {
		var studentID, email string
		var completion StudentListCompletion
		if err := gradedCompletions.Scan(&studentID, &email, &completion.ListTitle, &completion.ItemTitle, &completion.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan graded completion: %w", err)
		}
		so := get(studentID, email)
		so.Completions = append(so.Completions, completion)
	}
	if err := gradedCompletions.Err(); err != nil {
		return nil, fmt.Errorf("iterate graded completions: %w", err)
	}
	overview := make([]StudentOverview, 0, len(order))
	for _, id := range order {
		so := byStudent[id]
		so.Stats = computeStudentStats(so.Challenges)
		so.WeeklyProgress = computeWeeklyProgress(so.Challenges)
		sort.Slice(so.Completions, func(i, j int) bool { return so.Completions[i].CompletedAt.After(so.Completions[j].CompletedAt) })
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
		stats.WrongAnswerCount += cs.WrongAnswerCount
		stats.RuntimeErrorCount += cs.RuntimeErrorCount
		stats.TimeoutCount += cs.TimeoutCount
		if cs.AcceptedCount == 0 {
			stats.UnresolvedChallenges++
			if cs.WrongAnswerCount+cs.RuntimeErrorCount+cs.TimeoutCount >= 3 {
				stats.RecurringDifficulties++
			}
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

func computeWeeklyProgress(challenges []StudentChallengeSummary) []StudentWeek {
	weeks := make(map[string]*StudentWeek)
	for _, challenge := range challenges {
		solved := false
		for _, attempt := range challenge.Attempts {
			date := attempt.CreatedAt.UTC()
			daysSinceMonday := (int(date.Weekday()) + 6) % 7
			key := date.AddDate(0, 0, -daysSinceMonday).Format("2006-01-02")
			if weeks[key] == nil {
				weeks[key] = &StudentWeek{WeekStart: key}
			}
			weeks[key].Attempts++
			if attempt.Verdict == "accepted" && !solved {
				weeks[key].NewlySolved++
				solved = true
			}
		}
	}
	progress := make([]StudentWeek, 0, len(weeks))
	for _, week := range weeks {
		progress = append(progress, *week)
	}
	sort.Slice(progress, func(i, j int) bool { return progress[i].WeekStart < progress[j].WeekStart })
	return progress
}
