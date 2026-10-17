package store

import (
	"context"
	"fmt"
	"strings"
)

// Historical summaries never include source code, snapshots or private tests.
// Collective verdicts remain separate from individual progress.
type LiveRoomOutcome struct {
	ListItemID     string  `json:"list_item_id"`
	Title          string  `json:"title"`
	Status         *string `json:"status"`
	Verdict        *string `json:"verdict"`
	PassedCount    *int    `json:"passed_count"`
	TotalTestCases *int    `json:"total_test_cases"`
}

type LiveSessionResults struct {
	Session            LiveSession           `json:"session"`
	Students           []LiveStudentProgress `json:"students"`
	Rounds             []LiveSessionRound    `json:"rounds"`
	Rooms              []LiveRoomOutcome     `json:"rooms"`
	PreparingResults   int                   `json:"preparing_results"`
	PendingSubmissions int                   `json:"pending_submissions"`
}

// LiveSessionResults reads existing records only. Do not use room creation,
// round expiration, or Yjs socket helpers while browsing archived sessions.
func (s *Store) LiveSessionResults(ctx context.Context, id, viewer string, teacher bool) (LiveSessionResults, error) {
	session, participants, err := s.GetLiveSession(ctx, id, viewer)
	if err != nil {
		return LiveSessionResults{}, fmt.Errorf("get results session: %w", err)
	}
	owner := teacher && session.CreatedBy == viewer
	if !owner && !session.Joined {
		return LiveSessionResults{}, ErrNotFound
	}
	if session.Status != "finished" {
		return LiveSessionResults{}, ErrConflict
	}
	result := LiveSessionResults{Session: session, Students: []LiveStudentProgress{}, Rounds: []LiveSessionRound{}, Rooms: []LiveRoomOutcome{}}
	byUser := map[string]int{}
	for _, participant := range participants {
		if !owner && participant.UserID != viewer {
			continue
		}
		byUser[participant.UserID] = len(result.Students)
		result.Students = append(result.Students, LiveStudentProgress{UserID: participant.UserID, Email: participant.Email, Challenges: map[string]LiveCell{}})
	}
	query := `SELECT user_id,challenge_id,count(*),bool_or(status='accepted'),count(*) FILTER (WHERE status='pending') FROM submissions WHERE live_session_id=$1 AND is_test_run=false AND live_room_id IS NULL AND user_id IS NOT NULL`
	args := []any{id}
	if !owner {
		query += ` AND user_id=$2`
		args = append(args, viewer)
	}
	rows, err := s.db.QueryContext(ctx, query+` GROUP BY user_id,challenge_id`, args...)
	if err != nil {
		return result, fmt.Errorf("query archived individual attempts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var userID, challengeID string
		var attempts, pending int
		var accepted bool
		if err := rows.Scan(&userID, &challengeID, &attempts, &accepted, &pending); err != nil {
			return result, fmt.Errorf("scan archived attempt: %w", err)
		}
		index, ok := byUser[userID]
		if !ok {
			continue
		}
		student := &result.Students[index]
		student.Challenges[challengeID] = LiveCell{ChallengeID: challengeID, Attempts: attempts, Accepted: accepted}
		student.Attempts += attempts
		if accepted {
			student.Accepted++
		}
		result.PendingSubmissions += pending
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate archived attempts: %w", err)
	}
	rows.Close()
	if session.Mode == "timed_challenge" {
		rounds, err := s.db.QueryContext(ctx, `SELECT `+liveRoundColumns+` FROM live_session_rounds WHERE session_id=$1 ORDER BY sequence`, id)
		if err != nil {
			return result, fmt.Errorf("query archived rounds: %w", err)
		}
		defer rounds.Close()
		for rounds.Next() {
			round, err := scanLiveRound(rounds)
			if err != nil {
				return result, fmt.Errorf("scan archived round: %w", err)
			}
			result.Rounds = append(result.Rounds, round)
		}
		if err := rounds.Err(); err != nil {
			return result, fmt.Errorf("iterate archived rounds: %w", err)
		}
	}
	if session.Mode == "collaborative" {
		rooms, err := s.db.QueryContext(ctx, `SELECT li.id,li.title,r.status,sub.status,sub.passed_count,sub.total_test_cases,r.text_snapshot
   FROM list_items li LEFT JOIN live_session_rooms r ON r.list_item_id=li.id AND r.session_id=$1
   LEFT JOIN LATERAL (SELECT status,passed_count,total_test_cases FROM submissions WHERE live_room_id=r.id AND is_test_run=false ORDER BY created_at DESC,id DESC LIMIT 1) sub ON true
   WHERE li.list_id=$2 AND li.linked_challenge_id IS NOT NULL ORDER BY li.ordinal`, id, session.ProblemListID)
		if err != nil {
			return result, fmt.Errorf("query archived shared results: %w", err)
		}
		defer rooms.Close()
		for rooms.Next() {
			var room LiveRoomOutcome
			var snapshot *string
			if err := rooms.Scan(&room.ListItemID, &room.Title, &room.Status, &room.Verdict, &room.PassedCount, &room.TotalTestCases, &snapshot); err != nil {
				return result, fmt.Errorf("scan archived shared result: %w", err)
			}
			// Finish commits session state before freezing rooms and creating
			// their submissions. Keep clients polling across that preparation gap.
			if room.Status != nil && (*room.Status == "open" || (room.Verdict == nil && snapshot != nil && strings.TrimSpace(*snapshot) != "")) {
				result.PreparingResults++
			}
			if room.Verdict != nil && *room.Verdict == "pending" {
				result.PendingSubmissions++
			}
			result.Rooms = append(result.Rooms, room)
		}
		if err := rooms.Err(); err != nil {
			return result, fmt.Errorf("iterate archived shared results: %w", err)
		}
	}
	return result, nil
}
