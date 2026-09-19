package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const LiveEventsChannel = "live_session_events"

type LiveSession struct {
	ID               string     `json:"id"`
	ProblemListID    string     `json:"problem_list_id"`
	CreatedBy        string     `json:"created_by"`
	Status           string     `json:"status"`
	Mode             string     `json:"mode"`
	MinParticipants  int        `json:"min_participants"`
	StartedAt        *time.Time `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at"`
	CreatedAt        time.Time  `json:"created_at"`
	Title            string     `json:"title"`
	TeacherEmail     string     `json:"teacher_email"`
	ParticipantCount int        `json:"participant_count"`
	Joined           bool       `json:"joined"`
	Items            []ListItem `json:"items"`
}
type LiveSessionRound struct {
	ID              string     `json:"id"`
	SessionID       string     `json:"session_id"`
	ListItemID      string     `json:"list_item_id"`
	DurationSeconds int        `json:"duration_seconds"`
	StartedAt       *time.Time `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
	Status          string     `json:"status"`
	Sequence        int        `json:"sequence"`
}
type LiveRoundStudentStatus struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Status string `json:"status"`
}

func (s *Store) PublishLiveEvent(ctx context.Context, sessionID, event string, data any) error {
	if s.rdb == nil {
		return nil
	}
	payload, err := json.Marshal(map[string]any{"session_id": sessionID, "type": event, "data": data})
	if err != nil {
		return err
	}
	return s.rdb.Publish(ctx, LiveEventsChannel, payload).Err()
}

func (s *Store) PublishLiveSubmissionUpdate(ctx context.Context, sessionID, userID, challengeID, status string) error {
	var roundID string
	err := s.db.QueryRowContext(ctx, `SELECT r.id FROM live_session_rounds r JOIN list_items li ON li.id=r.list_item_id WHERE r.session_id=$1 AND r.status='active' AND li.linked_challenge_id=$2`, sessionID, challengeID).Scan(&roundID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.PublishLiveEvent(ctx, sessionID, "submission_update", map[string]string{"round_id": roundID, "user_id": userID, "status": status})
}

type LiveParticipant struct {
	UserID   string    `json:"user_id"`
	Email    string    `json:"email"`
	JoinedAt time.Time `json:"joined_at"`
}
type LiveCell struct {
	ChallengeID string `json:"challenge_id"`
	Attempts    int    `json:"attempts"`
	Accepted    bool   `json:"accepted"`
}
type LiveStudentProgress struct {
	UserID     string              `json:"user_id"`
	Email      string              `json:"email"`
	Challenges map[string]LiveCell `json:"challenges"`
	Attempts   int                 `json:"attempts"`
	Accepted   int                 `json:"accepted"`
}
type LiveDashboard struct {
	Session           LiveSession           `json:"session"`
	Participants      []LiveParticipant     `json:"participants"`
	Students          []LiveStudentProgress `json:"students"`
	ChallengeAttempts map[string]int        `json:"challenge_attempts"`
	ChallengeAccepted map[string]int        `json:"challenge_accepted"`
}

func scanLive(row interface{ Scan(...any) error }) (LiveSession, error) {
	var x LiveSession
	err := row.Scan(&x.ID, &x.ProblemListID, &x.CreatedBy, &x.Status, &x.Mode, &x.MinParticipants, &x.StartedAt, &x.FinishedAt, &x.CreatedAt, &x.Title, &x.TeacherEmail, &x.ParticipantCount)
	return x, err
}

const liveColumns = `s.id,s.problem_list_id,s.created_by,s.status,s.mode,s.min_participants,s.started_at,s.finished_at,s.created_at,l.title,u.email,(SELECT count(*) FROM live_session_participants p WHERE p.session_id=s.id)`

func (s *Store) CreateLiveSession(ctx context.Context, listID, teacherID string, min int) (LiveSession, error) {
	return s.CreateLiveSessionWithMode(ctx, listID, teacherID, min, "individual")
}
func (s *Store) CreateLiveSessionWithMode(ctx context.Context, listID, teacherID string, min int, mode string) (LiveSession, error) {
	if mode != "individual" && mode != "timed_challenge" && mode != "collaborative" {
		return LiveSession{}, ErrConflict
	}
	var owned bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM problem_lists WHERE id=$1 AND teacher_id=$2)`, listID, teacherID).Scan(&owned); err != nil {
		return LiveSession{}, err
	}
	if !owned {
		return LiveSession{}, ErrNotFound
	}
	var executable int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM list_items WHERE list_id=$1 AND linked_challenge_id IS NOT NULL`, listID).Scan(&executable); err != nil {
		return LiveSession{}, err
	}
	if executable == 0 {
		return LiveSession{}, ErrConflict
	}
	row := s.db.QueryRowContext(ctx, `WITH new_session AS (INSERT INTO live_sessions(problem_list_id,created_by,min_participants,mode) VALUES($1,$2,$3,$4) RETURNING *) SELECT `+liveColumns+` FROM new_session s JOIN problem_lists l ON l.id=s.problem_list_id JOIN users u ON u.id=s.created_by`, listID, teacherID, min, mode)
	return scanLive(row)
}
func (s *Store) ListLiveSessions(ctx context.Context, viewerID string, teacher bool) ([]LiveSession, error) {
	return s.listLiveSessions(ctx, viewerID, teacher)
}
func (s *Store) listLiveSessions(ctx context.Context, viewerID string, teacher bool) ([]LiveSession, error) {
	q := `SELECT ` + liveColumns + `, EXISTS(SELECT 1 FROM live_session_participants p WHERE p.session_id=s.id AND p.user_id=$1) FROM live_sessions s JOIN problem_lists l ON l.id=s.problem_list_id JOIN users u ON u.id=s.created_by WHERE s.status <> 'finished'`
	if teacher {
		q += ` AND s.created_by=$1`
	}
	q += ` ORDER BY s.created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LiveSession{}
	for rows.Next() {
		var x LiveSession
		if err := rows.Scan(&x.ID, &x.ProblemListID, &x.CreatedBy, &x.Status, &x.Mode, &x.MinParticipants, &x.StartedAt, &x.FinishedAt, &x.CreatedAt, &x.Title, &x.TeacherEmail, &x.ParticipantCount, &x.Joined); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Store) GetLiveSession(ctx context.Context, id, viewer string) (LiveSession, []LiveParticipant, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+liveColumns+`, EXISTS(SELECT 1 FROM live_session_participants p WHERE p.session_id=s.id AND p.user_id=$1) FROM live_sessions s JOIN problem_lists l ON l.id=s.problem_list_id JOIN users u ON u.id=s.created_by WHERE s.id=$2`, viewer, id)
	var x LiveSession
	if err := row.Scan(&x.ID, &x.ProblemListID, &x.CreatedBy, &x.Status, &x.Mode, &x.MinParticipants, &x.StartedAt, &x.FinishedAt, &x.CreatedAt, &x.Title, &x.TeacherEmail, &x.ParticipantCount, &x.Joined); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return x, nil, ErrNotFound
		}
		return x, nil, err
	}
	items, err := s.liveItems(ctx, x.ProblemListID)
	if err != nil {
		return x, nil, err
	}
	x.Items = items
	rows, err := s.db.QueryContext(ctx, `SELECT p.user_id,u.email,p.joined_at FROM live_session_participants p JOIN users u ON u.id=p.user_id WHERE p.session_id=$1 ORDER BY p.joined_at`, id)
	if err != nil {
		return x, nil, err
	}
	defer rows.Close()
	ps := []LiveParticipant{}
	for rows.Next() {
		var p LiveParticipant
		if err := rows.Scan(&p.UserID, &p.Email, &p.JoinedAt); err != nil {
			return x, nil, err
		}
		ps = append(ps, p)
	}
	return x, ps, rows.Err()
}
func (s *Store) liveItems(ctx context.Context, listID string) ([]ListItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT li.id,li.list_id,li.ordinal,li.title,li.difficulty,li.is_bonus,li.body,li.linked_challenge_id,c.title,c.slug,li.created_at,li.updated_at FROM list_items li JOIN challenges c ON c.id=li.linked_challenge_id WHERE li.list_id=$1 ORDER BY li.ordinal`, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ListItem{}
	for rows.Next() {
		var i ListItem
		if err := rows.Scan(&i.ID, &i.ListID, &i.Ordinal, &i.Title, &i.Difficulty, &i.IsBonus, &i.Body, &i.LinkedChallengeID, &i.ChallengeTitle, &i.ChallengeSlug, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
func (s *Store) JoinLiveSession(ctx context.Context, id, user string) (LiveSession, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LiveSession{}, err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM live_sessions WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LiveSession{}, ErrNotFound
		}
		return LiveSession{}, err
	}
	if status == "finished" {
		return LiveSession{}, ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO live_session_participants(session_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, user); err != nil {
		return LiveSession{}, err
	}
	if status == "waiting" {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM live_session_participants WHERE session_id=$1`, id).Scan(&n); err != nil {
			return LiveSession{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE live_sessions SET status='active',started_at=now() WHERE id=$1 AND min_participants <= $2`, id, n); err != nil {
			return LiveSession{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return LiveSession{}, err
	}
	x, _, err := s.GetLiveSession(ctx, id, user)
	return x, err
}
func (s *Store) FinishLiveSession(ctx context.Context, id, teacher string) error {
	r, err := s.db.ExecContext(ctx, `UPDATE live_sessions SET status='finished',finished_at=now() WHERE id=$1 AND created_by=$2 AND status <> 'finished'`, id, teacher)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) ValidateLiveSubmission(ctx context.Context, session, user, challenge string) error {
	if _, err := s.ExpireLiveSessionRounds(ctx, session); err != nil {
		return err
	}
	var ok bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM live_sessions s JOIN live_session_participants p ON p.session_id=s.id JOIN list_items li ON li.list_id=s.problem_list_id LEFT JOIN live_session_rounds r ON r.session_id=s.id AND r.list_item_id=li.id AND r.status='active' WHERE s.id=$1 AND s.status='active' AND p.user_id=$2 AND li.linked_challenge_id=$3 AND (s.mode <> 'timed_challenge' OR r.id IS NOT NULL))`, session, user, challenge).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}
func (s *Store) LiveDashboard(ctx context.Context, id, teacherID string) (LiveDashboard, error) {
	x, ps, err := s.GetLiveSession(ctx, id, teacherID)
	if err != nil {
		return LiveDashboard{}, err
	}
	if x.CreatedBy != teacherID {
		return LiveDashboard{}, ErrNotFound
	}
	d := LiveDashboard{Session: x, Participants: ps, Students: []LiveStudentProgress{}, ChallengeAttempts: map[string]int{}, ChallengeAccepted: map[string]int{}}
	by := map[string]*LiveStudentProgress{}
	for _, p := range ps {
		v := &LiveStudentProgress{UserID: p.UserID, Email: p.Email, Challenges: map[string]LiveCell{}}
		by[p.UserID] = v
		d.Students = append(d.Students, *v)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT user_id,challenge_id,count(*),bool_or(status='accepted') FROM submissions WHERE live_session_id=$1 GROUP BY user_id,challenge_id`, id)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var u, c string
		var n int
		var ok bool
		if err := rows.Scan(&u, &c, &n, &ok); err != nil {
			return d, err
		}
		v := by[u]
		if v == nil {
			continue
		}
		v.Challenges[c] = LiveCell{ChallengeID: c, Attempts: n, Accepted: ok}
		v.Attempts += n
		if ok {
			v.Accepted++
		}
		d.ChallengeAttempts[c] += n
		if ok {
			d.ChallengeAccepted[c]++
		}
	}
	d.Students = []LiveStudentProgress{}
	for _, p := range ps {
		d.Students = append(d.Students, *by[p.UserID])
	}
	return d, rows.Err()
}

const liveRoundColumns = `id,session_id,list_item_id,duration_seconds,started_at,ended_at,status,sequence`

func scanLiveRound(row interface{ Scan(...any) error }) (LiveSessionRound, error) {
	var round LiveSessionRound
	err := row.Scan(&round.ID, &round.SessionID, &round.ListItemID, &round.DurationSeconds, &round.StartedAt, &round.EndedAt, &round.Status, &round.Sequence)
	return round, err
}

// CreateLiveSessionRounds snapshots the executable list items in their list order.
func (s *Store) CreateLiveSessionRounds(ctx context.Context, sessionID, teacherID string) ([]LiveSessionRound, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var listID string
	var mode string
	if err := tx.QueryRowContext(ctx, `SELECT problem_list_id,mode FROM live_sessions WHERE id=$1 AND created_by=$2 FOR UPDATE`, sessionID, teacherID).Scan(&listID, &mode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if mode != "timed_challenge" {
		return nil, ErrConflict
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM live_session_rounds WHERE session_id=$1)`, sessionID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return s.LiveSessionRounds(ctx, sessionID, teacherID)
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM list_items WHERE list_id=$1 AND linked_challenge_id IS NOT NULL ORDER BY ordinal`, listID)
	if err != nil {
		return nil, err
	}
	itemIDs := []string{}
	for rows.Next() {
		var itemID string
		if err := rows.Scan(&itemID); err != nil {
			return nil, err
		}
		itemIDs = append(itemIDs, itemID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(itemIDs) == 0 {
		return nil, ErrConflict
	}
	for sequence, itemID := range itemIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO live_session_rounds(session_id,list_item_id,sequence) VALUES($1,$2,$3)`, sessionID, itemID, sequence); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.LiveSessionRounds(ctx, sessionID, teacherID)
}

func (s *Store) LiveSessionRounds(ctx context.Context, sessionID, teacherID string) ([]LiveSessionRound, error) {
	var owner bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM live_sessions WHERE id=$1 AND created_by=$2)`, sessionID, teacherID).Scan(&owner); err != nil {
		return nil, err
	}
	if !owner {
		return nil, ErrNotFound
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+liveRoundColumns+` FROM live_session_rounds WHERE session_id=$1 ORDER BY sequence`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LiveSessionRound{}
	for rows.Next() {
		r, err := scanLiveRound(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) StartLiveSessionRound(ctx context.Context, sessionID, roundID, teacherID string, seconds int) (LiveSessionRound, error) {
	if seconds <= 0 {
		return LiveSessionRound{}, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LiveSessionRound{}, err
	}
	defer tx.Rollback()
	var owner bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM live_sessions WHERE id=$1 AND created_by=$2 AND mode='timed_challenge')`, sessionID, teacherID).Scan(&owner); err != nil {
		return LiveSessionRound{}, err
	}
	if !owner {
		return LiveSessionRound{}, ErrNotFound
	}
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM live_session_rounds WHERE session_id=$1 AND status='active')`, sessionID).Scan(&active); err != nil {
		return LiveSessionRound{}, err
	}
	if active {
		return LiveSessionRound{}, ErrConflict
	}
	row := tx.QueryRowContext(ctx, `UPDATE live_session_rounds SET status='active',started_at=now(),ended_at=NULL,duration_seconds=$1 WHERE id=$2 AND session_id=$3 AND status='pending' RETURNING `+liveRoundColumns, seconds, roundID, sessionID)
	round, err := scanLiveRound(row)
	if errors.Is(err, sql.ErrNoRows) {
		return LiveSessionRound{}, ErrNotFound
	}
	if err != nil {
		return LiveSessionRound{}, err
	}
	if err := tx.Commit(); err != nil {
		return LiveSessionRound{}, err
	}
	return round, nil
}

// EndLiveSessionRound is idempotent for an already-ended round.
func (s *Store) EndLiveSessionRound(ctx context.Context, sessionID, roundID, teacherID string) (LiveSessionRound, error) {
	row := s.db.QueryRowContext(ctx, `UPDATE live_session_rounds r SET status='ended',ended_at=COALESCE(r.ended_at,now()) FROM live_sessions s WHERE r.id=$1 AND r.session_id=$2 AND s.id=r.session_id AND s.created_by=$3 AND r.status='active' RETURNING r.id,r.session_id,r.list_item_id,r.duration_seconds,r.started_at,r.ended_at,r.status,r.sequence`, roundID, sessionID, teacherID)
	round, err := scanLiveRound(row)
	if errors.Is(err, sql.ErrNoRows) {
		return LiveSessionRound{}, ErrNotFound
	}
	return round, err
}

func (s *Store) ExpireLiveSessionRounds(ctx context.Context, sessionID string) ([]LiveSessionRound, error) {
	rows, err := s.db.QueryContext(ctx, `UPDATE live_session_rounds SET status='ended',ended_at=now() WHERE session_id=$1 AND status='active' AND started_at + (duration_seconds * INTERVAL '1 second') <= now() RETURNING `+liveRoundColumns, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LiveSessionRound{}
	for rows.Next() {
		r, err := scanLiveRound(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CurrentLiveSessionRound(ctx context.Context, sessionID, userID string) (*LiveSessionRound, error) {
	if _, err := s.ExpireLiveSessionRounds(ctx, sessionID); err != nil {
		return nil, err
	}
	var allowed bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM live_sessions s LEFT JOIN live_session_participants p ON p.session_id=s.id WHERE s.id=$1 AND (s.created_by=$2 OR p.user_id=$2))`, sessionID, userID).Scan(&allowed); err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrNotFound
	}
	row := s.db.QueryRowContext(ctx, `SELECT `+liveRoundColumns+` FROM live_session_rounds WHERE session_id=$1 ORDER BY CASE WHEN status='active' THEN 0 WHEN status='ended' THEN 1 ELSE 2 END, sequence DESC LIMIT 1`, sessionID)
	r, err := scanLiveRound(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) LiveRoundStatus(ctx context.Context, sessionID, roundID, teacherID string) (LiveSessionRound, []LiveRoundStudentStatus, error) {
	if _, err := s.ExpireLiveSessionRounds(ctx, sessionID); err != nil {
		return LiveSessionRound{}, nil, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT r.id,r.session_id,r.list_item_id,r.duration_seconds,r.started_at,r.ended_at,r.status,r.sequence FROM live_session_rounds r JOIN live_sessions s ON s.id=r.session_id WHERE r.id=$1 AND r.session_id=$2 AND s.created_by=$3`, roundID, sessionID, teacherID)
	round, err := scanLiveRound(row)
	if errors.Is(err, sql.ErrNoRows) {
		return round, nil, ErrNotFound
	}
	if err != nil {
		return round, nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT p.user_id,u.email,CASE WHEN bool_or(sub.status='accepted') THEN 'passed' WHEN bool_or(sub.status='pending') THEN 'submitted_pending' WHEN count(sub.id)>0 THEN 'failed' ELSE 'not_started' END FROM live_session_rounds r JOIN list_items li ON li.id=r.list_item_id JOIN live_session_participants p ON p.session_id=r.session_id JOIN users u ON u.id=p.user_id LEFT JOIN submissions sub ON sub.user_id=p.user_id AND sub.live_session_id=r.session_id AND sub.challenge_id=li.linked_challenge_id AND sub.created_at >= r.started_at WHERE r.id=$1 AND r.session_id=$2 GROUP BY p.user_id,u.email ORDER BY u.email`, roundID, sessionID)
	if err != nil {
		return round, nil, err
	}
	defer rows.Close()
	out := []LiveRoundStudentStatus{}
	for rows.Next() {
		var x LiveRoundStudentStatus
		if err := rows.Scan(&x.UserID, &x.Email, &x.Status); err != nil {
			return round, nil, err
		}
		out = append(out, x)
	}
	return round, out, rows.Err()
}
