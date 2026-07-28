package store

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type StudentChallengeSummary struct {
	ChallengeID    string `json:"challenge_id"`
	ChallengeTitle string `json:"challenge_title"`
	BestVerdict    string `json:"best_verdict"`
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
	Challenges  []StudentChallengeSummary `json:"challenges"`
	Completions []StudentListCompletion   `json:"list_completions"`
}

// TeacherStudentsOverview returns every student (role=student) who has at
// least one non-test-run submission or one list item completion. Each
// challenge the student has submitted to is annotated with their best
// verdict: "accepted" if they ever got one (ignoring test runs), otherwise
// the status of their most recent real submission. Students with neither a
// real submission nor a completion are omitted entirely.
func (s *Store) TeacherStudentsOverview(ctx context.Context) ([]StudentOverview, error) {
	byStudent := make(map[string]*StudentOverview)
	order := make([]string, 0)

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

	challengeRows, err := s.db.QueryContext(ctx, `
		WITH ranked AS (
			SELECT sub.user_id, u.email, sub.challenge_id, ch.title AS challenge_title,
			       sub.status, sub.created_at,
			       ROW_NUMBER() OVER (PARTITION BY sub.user_id, sub.challenge_id ORDER BY sub.created_at DESC) AS rn,
			       BOOL_OR(sub.status = 'accepted') OVER (PARTITION BY sub.user_id, sub.challenge_id) AS has_accepted
			FROM submissions sub
			JOIN challenges ch ON ch.id = sub.challenge_id
			JOIN users u ON u.id = sub.user_id AND u.role = 'student'
			WHERE sub.is_test_run = false
		)
		SELECT user_id, email, challenge_id, challenge_title,
		       CASE WHEN has_accepted THEN 'accepted' ELSE status END AS best_verdict
		FROM ranked
		WHERE rn = 1`)
	if err != nil {
		return nil, fmt.Errorf("query student challenge verdicts: %w", err)
	}
	defer challengeRows.Close()
	for challengeRows.Next() {
		var studentID, email, challengeID, title, verdict string
		if err := challengeRows.Scan(&studentID, &email, &challengeID, &title, &verdict); err != nil {
			return nil, fmt.Errorf("scan student challenge verdict: %w", err)
		}
		so := get(studentID, email)
		so.Challenges = append(so.Challenges, StudentChallengeSummary{
			ChallengeID: challengeID, ChallengeTitle: title, BestVerdict: verdict,
		})
	}
	if err := challengeRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate student challenge verdicts: %w", err)
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
		overview = append(overview, *byStudent[id])
	}
	sort.Slice(overview, func(i, j int) bool { return overview[i].Email < overview[j].Email })
	return overview, nil
}
