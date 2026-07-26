package worker

import (
	"context"
	"testing"
)

// scriptedExecutor replays a fixed sequence of RunResult/error pairs, one per
// call, so evaluate can be driven through the fail-fast loop without Docker.
type scriptedExecutor struct {
	results []RunResult
	errs    []error
	calls   int
}

func (e *scriptedExecutor) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	i := e.calls
	e.calls++
	if i >= len(e.results) {
		return RunResult{}, nil
	}
	return e.results[i], e.errs[i]
}

// okCases builds n test cases that all expect the output "ok".
func okCases(n int) []testCaseRecord {
	tcs := make([]testCaseRecord, n)
	for i := range tcs {
		tcs[i] = testCaseRecord{Input: "x", ExpectedOutput: "ok"}
	}
	return tcs
}

func TestEvaluate_CountsPassedWhenFailingMidway(t *testing.T) {
	exec := &scriptedExecutor{
		results: []RunResult{{Stdout: "ok"}, {Stdout: "ok"}, {Stdout: "ok"}, {Stdout: "nope"}},
		errs:    []error{nil, nil, nil, nil},
	}

	res := evaluate(context.Background(), exec, "python", "code", okCases(5), 1000, 128)

	if res.status != "wrong_answer" {
		t.Errorf("status = %q, want wrong_answer", res.status)
	}
	if res.passedCount != 3 {
		t.Errorf("passedCount = %d, want 3", res.passedCount)
	}
	if res.total != 5 {
		t.Errorf("total = %d, want 5", res.total)
	}
}

func TestEvaluate_CountsAllPassed(t *testing.T) {
	exec := &scriptedExecutor{
		results: []RunResult{{Stdout: "ok"}, {Stdout: "ok"}, {Stdout: "ok"}, {Stdout: "ok"}, {Stdout: "ok"}},
		errs:    make([]error, 5),
	}

	res := evaluate(context.Background(), exec, "python", "code", okCases(5), 1000, 128)

	if res.status != "accepted" {
		t.Errorf("status = %q, want accepted", res.status)
	}
	if res.passedCount != 5 {
		t.Errorf("passedCount = %d, want 5", res.passedCount)
	}
	if res.total != 5 {
		t.Errorf("total = %d, want 5", res.total)
	}
}

// A failure on the very first case (e.g. a compile error surfacing as a
// non-zero exit from `go run`) means nothing passed: 0 of N, runtime_error.
// This is distinct from the worker failing before the loop at all, which
// leaves passed_count/total_test_cases NULL by never calling the update.
func TestEvaluate_FirstCaseErrorCountsZero(t *testing.T) {
	exec := &scriptedExecutor{
		results: []RunResult{{Stderr: "boom"}},
		errs:    []error{ErrRuntimeError},
	}

	res := evaluate(context.Background(), exec, "go", "code", okCases(5), 1000, 128)

	if res.status != "runtime_error" {
		t.Errorf("status = %q, want runtime_error", res.status)
	}
	if res.passedCount != 0 {
		t.Errorf("passedCount = %d, want 0", res.passedCount)
	}
	if res.total != 5 {
		t.Errorf("total = %d, want 5", res.total)
	}
	if res.stderr != "boom" {
		t.Errorf("stderr = %q, want boom", res.stderr)
	}
}
