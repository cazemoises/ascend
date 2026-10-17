package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Assessment contains only public question data. Answer keys are deliberately
// absent from this type and from all list/item and student response DTOs.
type Assessment struct {
	ID            string    `json:"id"`
	ListItemID    string    `json:"list_item_id"`
	Revision      int       `json:"revision"`
	Kind          string    `json:"kind"`
	Prompt        string    `json:"prompt"`
	Options       []string  `json:"options"`
	CaseSensitive bool      `json:"case_sensitive"`
	CreatedAt     time.Time `json:"created_at"`
}

type AssessmentAuthoring struct {
	Assessment
	AcceptedAnswers []string `json:"accepted_answers"`
}

type AssessmentInput struct {
	Kind            string   `json:"kind"`
	Prompt          string   `json:"prompt"`
	Options         []string `json:"options"`
	AcceptedAnswers []string `json:"accepted_answers"`
	CaseSensitive   bool     `json:"case_sensitive"`
}

func (in AssessmentInput) Validate() error {
	if strings.TrimSpace(in.Prompt) == "" || len(in.Prompt) > 20000 {
		return errors.New("prompt is required (up to 20000 bytes)")
	}
	if in.Kind != "multiple_choice" && in.Kind != "short_answer" {
		return errors.New("kind must be multiple_choice or short_answer")
	}
	if len(in.AcceptedAnswers) == 0 || len(in.AcceptedAnswers) > 10 {
		return errors.New("provide 1 to 10 accepted answers")
	}
	for _, answer := range in.AcceptedAnswers {
		if strings.TrimSpace(answer) == "" || len(answer) > 2000 {
			return errors.New("accepted answers must contain 1 to 2000 bytes")
		}
	}
	if in.Kind == "short_answer" {
		if len(in.Options) != 0 {
			return errors.New("short answers cannot have options")
		}
		return nil
	}
	if len(in.Options) < 2 || len(in.Options) > 10 || len(in.AcceptedAnswers) != 1 {
		return errors.New("multiple choice requires 2 to 10 options and exactly one accepted answer")
	}
	seen := map[string]bool{}
	for _, option := range in.Options {
		if strings.TrimSpace(option) == "" || len(option) > 1000 || seen[option] {
			return errors.New("options must be distinct and contain 1 to 1000 bytes")
		}
		seen[option] = true
	}
	if !seen[in.AcceptedAnswers[0]] {
		return errors.New("accepted answer must be one of the options")
	}
	return nil
}

func assessmentAccepts(kind string, caseSensitive bool, keys []string, answer string) bool {
	if kind == "multiple_choice" {
		return len(keys) == 1 && answer == keys[0]
	}
	normalized := strings.Join(strings.Fields(answer), " ")
	for _, key := range keys {
		key = strings.Join(strings.Fields(key), " ")
		if (caseSensitive && normalized == key) || (!caseSensitive && strings.EqualFold(normalized, key)) {
			return true
		}
	}
	return false
}

const assessmentColumns = `ar.id,ar.list_item_id,ar.revision,ar.kind,ar.prompt,ar.options,ar.case_sensitive,ar.created_at`

func scanAssessment(row interface{ Scan(...any) error }, withKey bool) (AssessmentAuthoring, error) {
	var a AssessmentAuthoring
	var options, answers []byte
	args := []any{&a.ID, &a.ListItemID, &a.Revision, &a.Kind, &a.Prompt, &options, &a.CaseSensitive, &a.CreatedAt}
	if withKey {
		args = append(args, &answers)
	}
	if err := row.Scan(args...); err != nil {
		return a, err
	}
	if err := json.Unmarshal(options, &a.Options); err != nil {
		return a, fmt.Errorf("decode assessment options: %w", err)
	}
	if withKey {
		if err := json.Unmarshal(answers, &a.AcceptedAnswers); err != nil {
			return a, fmt.Errorf("decode assessment answer key: %w", err)
		}
	}
	return a, nil
}

func (s *Store) GetAssessment(ctx context.Context, itemID, viewer string, teacher, authoring bool) (AssessmentAuthoring, error) {
	columns := assessmentColumns
	if authoring {
		columns += `,ar.accepted_answers`
	}
	row := s.db.QueryRowContext(ctx, `SELECT `+columns+` FROM list_assessment_revisions ar JOIN list_items li ON li.id=ar.list_item_id JOIN problem_lists pl ON pl.id=li.list_id
 WHERE li.id=$1 AND (($4 AND $3 AND pl.teacher_id::text=$2) OR (NOT $4 AND (pl.published OR ($3 AND pl.teacher_id::text=$2)))) ORDER BY ar.revision DESC LIMIT 1`, itemID, viewer, teacher, authoring)
	a, err := scanAssessment(row, authoring)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, fmt.Errorf("get assessment: %w", err)
	}
	return a, nil
}

func (s *Store) SaveAssessment(ctx context.Context, itemID, teacherID string, in AssessmentInput) (AssessmentAuthoring, error) {
	if err := in.Validate(); err != nil {
		return AssessmentAuthoring{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AssessmentAuthoring{}, fmt.Errorf("begin assessment revision: %w", err)
	}
	defer tx.Rollback()
	var linked bool
	err = tx.QueryRowContext(ctx, `SELECT li.linked_challenge_id IS NOT NULL FROM list_items li JOIN problem_lists pl ON pl.id=li.list_id WHERE li.id=$1 AND pl.teacher_id=$2 FOR UPDATE OF li`, itemID, teacherID).Scan(&linked)
	if errors.Is(err, sql.ErrNoRows) {
		return AssessmentAuthoring{}, ErrNotFound
	}
	if err != nil {
		return AssessmentAuthoring{}, fmt.Errorf("lock assessment item: %w", err)
	}
	if linked {
		return AssessmentAuthoring{}, ErrLinkedItem
	}
	options := in.Options
	if options == nil {
		options = []string{}
	}
	current, currentErr := scanAssessment(tx.QueryRowContext(ctx, `SELECT `+assessmentColumns+`,ar.accepted_answers FROM list_assessment_revisions ar WHERE list_item_id=$1 ORDER BY revision DESC LIMIT 1`, itemID), true)
	if currentErr != nil && !errors.Is(currentErr, sql.ErrNoRows) {
		return AssessmentAuthoring{}, fmt.Errorf("read current assessment: %w", currentErr)
	}
	if currentErr == nil && current.Kind == in.Kind && current.Prompt == in.Prompt && current.CaseSensitive == in.CaseSensitive && slices.Equal(current.Options, options) && slices.Equal(current.AcceptedAnswers, in.AcceptedAnswers) {
		if err := tx.Commit(); err != nil {
			return current, fmt.Errorf("commit unchanged assessment: %w", err)
		}
		return current, nil
	}
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return AssessmentAuthoring{}, fmt.Errorf("encode assessment options: %w", err)
	}
	answersJSON, err := json.Marshal(in.AcceptedAnswers)
	if err != nil {
		return AssessmentAuthoring{}, fmt.Errorf("encode assessment answers: %w", err)
	}
	row := tx.QueryRowContext(ctx, `INSERT INTO list_assessment_revisions AS ar (list_item_id,revision,kind,prompt,options,accepted_answers,case_sensitive)
 SELECT $1,COALESCE(MAX(revision),0)+1,$2,$3,$4,$5,$6 FROM list_assessment_revisions WHERE list_item_id=$1 RETURNING `+assessmentColumns+`,ar.accepted_answers`, itemID, in.Kind, in.Prompt, string(optionsJSON), string(answersJSON), in.CaseSensitive)
	a, err := scanAssessment(row, true)
	if err != nil {
		return a, fmt.Errorf("insert assessment revision: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return a, fmt.Errorf("commit assessment revision: %w", err)
	}
	return a, nil
}

type AssessmentAttempt struct {
	ID        string     `json:"id"`
	Exercise  Assessment `json:"exercise"`
	Answer    string     `json:"answer"`
	Accepted  bool       `json:"accepted"`
	CreatedAt time.Time  `json:"created_at"`
}

func (s *Store) AnswerAssessment(ctx context.Context, itemID, studentID, revisionID, answer string) (AssessmentAttempt, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AssessmentAttempt{}, fmt.Errorf("begin assessment answer: %w", err)
	}
	defer tx.Rollback()
	var published bool
	err = tx.QueryRowContext(ctx, `SELECT pl.published FROM list_items li JOIN problem_lists pl ON pl.id=li.list_id WHERE li.id=$1 FOR SHARE OF li,pl`, itemID).Scan(&published)
	if errors.Is(err, sql.ErrNoRows) {
		return AssessmentAttempt{}, ErrNotFound
	}
	if err != nil {
		return AssessmentAttempt{}, fmt.Errorf("lock answering item: %w", err)
	}
	if !published {
		return AssessmentAttempt{}, ErrNotFound
	}
	a, err := scanAssessment(tx.QueryRowContext(ctx, `SELECT `+assessmentColumns+`,ar.accepted_answers FROM list_assessment_revisions ar WHERE ar.list_item_id=$1 ORDER BY revision DESC LIMIT 1`, itemID), true)
	if errors.Is(err, sql.ErrNoRows) {
		return AssessmentAttempt{}, ErrNotFound
	}
	if err != nil {
		return AssessmentAttempt{}, fmt.Errorf("read answering revision: %w", err)
	}
	if a.ID != revisionID {
		return AssessmentAttempt{}, ErrConflict
	}
	accepted := assessmentAccepts(a.Kind, a.CaseSensitive, a.AcceptedAnswers, answer)
	result := AssessmentAttempt{Exercise: a.Assessment, Answer: answer, Accepted: accepted}
	err = tx.QueryRowContext(ctx, `INSERT INTO list_assessment_attempts(revision_id,student_id,answer,accepted) VALUES($1,$2,$3,$4) RETURNING id,created_at`, a.ID, studentID, answer, accepted).Scan(&result.ID, &result.CreatedAt)
	if err != nil {
		return result, fmt.Errorf("insert assessment attempt: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit assessment attempt: %w", err)
	}
	return result, nil
}

func (s *Store) AssessmentAttempts(ctx context.Context, itemID, studentID string) ([]AssessmentAttempt, error) {
	if _, err := s.GetAssessment(ctx, itemID, studentID, false, false); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.answer,a.accepted,a.created_at,`+assessmentColumns+` FROM list_assessment_attempts a JOIN list_assessment_revisions ar ON ar.id=a.revision_id WHERE ar.list_item_id=$1 AND a.student_id=$2 ORDER BY a.created_at DESC,a.id DESC LIMIT 50`, itemID, studentID)
	if err != nil {
		return nil, fmt.Errorf("query own assessment attempts: %w", err)
	}
	defer rows.Close()
	attempts := []AssessmentAttempt{}
	for rows.Next() {
		var a AssessmentAttempt
		var options []byte
		if err := rows.Scan(&a.ID, &a.Answer, &a.Accepted, &a.CreatedAt, &a.Exercise.ID, &a.Exercise.ListItemID, &a.Exercise.Revision, &a.Exercise.Kind, &a.Exercise.Prompt, &options, &a.Exercise.CaseSensitive, &a.Exercise.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan assessment attempt: %w", err)
		}
		if err := json.Unmarshal(options, &a.Exercise.Options); err != nil {
			return nil, fmt.Errorf("decode attempted options: %w", err)
		}
		attempts = append(attempts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate own assessment attempts: %w", err)
	}
	return attempts, nil
}
