package store

import (
	"reflect"
	"testing"
	"time"
)

func TestPedagogicalDifficultyStats(t *testing.T) {
	tests := []struct {
		name                  string
		challenge             StudentChallengeSummary
		unresolved, recurring int
	}{
		{"pending is not failure", StudentChallengeSummary{TotalAttempts: 4}, 1, 0},
		{"below threshold", StudentChallengeSummary{WrongAnswerCount: 2}, 1, 0},
		{"mixed failures", StudentChallengeSummary{WrongAnswerCount: 1, RuntimeErrorCount: 1, TimeoutCount: 1}, 1, 1},
		{"resolved difficulty", StudentChallengeSummary{AcceptedCount: 1, WrongAnswerCount: 4}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeStudentStats([]StudentChallengeSummary{tt.challenge})
			if got.UnresolvedChallenges != tt.unresolved || got.RecurringDifficulties != tt.recurring {
				t.Fatalf("unresolved/recurring = %d/%d, want %d/%d", got.UnresolvedChallenges, got.RecurringDifficulties, tt.unresolved, tt.recurring)
			}
			if got.WrongAnswerCount != tt.challenge.WrongAnswerCount || got.RuntimeErrorCount != tt.challenge.RuntimeErrorCount || got.TimeoutCount != tt.challenge.TimeoutCount {
				t.Fatalf("failure totals mismatch: %+v", got)
			}
		})
	}
}

func TestWeeklyProgress(t *testing.T) {
	attempt := func(date, verdict string) SubmissionAttempt {
		at, err := time.Parse(time.RFC3339, date)
		if err != nil {
			t.Fatal(err)
		}
		return SubmissionAttempt{CreatedAt: at, Verdict: verdict}
	}
	tests := []struct {
		name       string
		challenges []StudentChallengeSummary
		want       []StudentWeek
	}{
		{"no submissions", nil, []StudentWeek{}},
		{"UTC boundary and first solve only", []StudentChallengeSummary{
			{Attempts: []SubmissionAttempt{
				attempt("2026-09-27T23:30:00-03:00", "wrong_answer"),
				attempt("2026-09-30T12:00:00Z", "accepted"),
				attempt("2026-10-05T12:00:00Z", "accepted"),
			}},
			{Attempts: []SubmissionAttempt{
				attempt("2026-09-27T23:00:00Z", "pending"),
				attempt("2026-10-05T13:00:00Z", "accepted"),
			}},
		}, []StudentWeek{{"2026-09-21", 1, 0}, {"2026-09-28", 2, 1}, {"2026-10-05", 2, 1}}},
		{"year boundary", []StudentChallengeSummary{{Attempts: []SubmissionAttempt{
			attempt("2027-01-01T00:00:00Z", "runtime_error"),
		}}}, []StudentWeek{{"2026-12-28", 1, 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeWeeklyProgress(tt.challenges)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
