package store

import (
	"strings"
	"testing"
)

func TestAssessmentValidation(t *testing.T) {
	tests := []struct {
		name  string
		input AssessmentInput
		valid bool
	}{
		{"choice", AssessmentInput{Kind: "multiple_choice", Prompt: "Pick", Options: []string{"A", "B"}, AcceptedAnswers: []string{"A"}}, true},
		{"duplicate options", AssessmentInput{Kind: "multiple_choice", Prompt: "Pick", Options: []string{"A", "A"}, AcceptedAnswers: []string{"A"}}, false},
		{"key outside choices", AssessmentInput{Kind: "multiple_choice", Prompt: "Pick", Options: []string{"A", "B"}, AcceptedAnswers: []string{"C"}}, false},
		{"multiple keys", AssessmentInput{Kind: "multiple_choice", Prompt: "Pick", Options: []string{"A", "B"}, AcceptedAnswers: []string{"A", "B"}}, false},
		{"short alternatives", AssessmentInput{Kind: "short_answer", Prompt: "Name", AcceptedAnswers: []string{"Go", "Golang"}}, true},
		{"blank key", AssessmentInput{Kind: "short_answer", Prompt: "Name", AcceptedAnswers: []string{" \t"}}, false},
		{"oversized key", AssessmentInput{Kind: "short_answer", Prompt: "Name", AcceptedAnswers: []string{strings.Repeat("a", 2001)}}, false},
		{"short options", AssessmentInput{Kind: "short_answer", Prompt: "Name", Options: []string{"A"}, AcceptedAnswers: []string{"A"}}, false},
		{"unknown kind", AssessmentInput{Kind: "essay", Prompt: "Name", AcceptedAnswers: []string{"A"}}, false},
		{"blank prompt", AssessmentInput{Kind: "short_answer", Prompt: " ", AcceptedAnswers: []string{"A"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.Validate() == nil; got != tt.valid {
				t.Fatalf("valid=%v, want %v", got, tt.valid)
			}
		})
	}
}
func TestAssessmentGrading(t *testing.T) {
	tests := []struct {
		name, kind string
		sensitive  bool
		keys       []string
		answer     string
		accepted   bool
	}{
		{"choice exact", "multiple_choice", false, []string{"A"}, "A", true},
		{"choice wrong case", "multiple_choice", false, []string{"A"}, "a", false},
		{"choice no trim", "multiple_choice", false, []string{"A"}, " A ", false},
		{"unicode whitespace and folding", "short_answer", false, []string{"São Paulo"}, "  SÃO\t\npaulo  ", true},
		{"alternative", "short_answer", false, []string{"Go", "Golang"}, "golang", true},
		{"sensitive", "short_answer", true, []string{"Go"}, "go", false},
		{"sensitive trim", "short_answer", true, []string{"Go"}, "  Go\t", true},
		{"accents preserved", "short_answer", false, []string{"café"}, "cafe", false},
		{"no regex", "short_answer", false, []string{"a.*"}, "abc", false},
		{"no substring", "short_answer", false, []string{"Go"}, "Golang", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := assessmentAccepts(tt.kind, tt.sensitive, tt.keys, tt.answer); got != tt.accepted {
				t.Fatalf("accepted=%v, want %v", got, tt.accepted)
			}
		})
	}
}
