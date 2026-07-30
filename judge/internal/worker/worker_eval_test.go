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

	res := evaluate(context.Background(), exec, "python", "code", "", okCases(5), 1000, 128)

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

	res := evaluate(context.Background(), exec, "python", "code", "", okCases(5), 1000, 128)

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

	res := evaluate(context.Background(), exec, "go", "code", "", okCases(5), 1000, 128)

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

// A wrong_answer verdict must carry the failing case's actual stdout and
// its expected output — the result page's diff display depends on both.
func TestEvaluate_WrongAnswerCapturesActualAndExpectedOutput(t *testing.T) {
	exec := &scriptedExecutor{
		results: []RunResult{{Stdout: "ok"}, {Stdout: "nope"}},
		errs:    []error{nil, nil},
	}
	cases := []testCaseRecord{
		{Input: "a", ExpectedOutput: "ok"},
		{Input: "b", ExpectedOutput: "definitely not nope"},
	}

	res := evaluate(context.Background(), exec, "python", "code", "", cases, 1000, 128)

	if res.status != "wrong_answer" {
		t.Fatalf("status = %q, want wrong_answer", res.status)
	}
	if res.stdout != "nope" {
		t.Errorf("stdout = %q, want %q", res.stdout, "nope")
	}
	if res.expectedOutput != "definitely not nope" {
		t.Errorf("expectedOutput = %q, want %q", res.expectedOutput, "definitely not nope")
	}
}

// An accepted run has no failing case, so expectedOutput must stay empty —
// the result page shows no "esperada" block on a clean pass. stdout,
// however, must carry the last-executed case's real output, so the result
// page can show what the accepted solution actually printed.
func TestEvaluate_AcceptedCapturesLastStdoutButNoExpectedOutput(t *testing.T) {
	exec := &scriptedExecutor{
		results: []RunResult{{Stdout: "ok"}, {Stdout: "ok"}, {Stdout: "final output"}},
		errs:    []error{nil, nil, nil},
	}
	cases := []testCaseRecord{
		{Input: "a", ExpectedOutput: "ok"},
		{Input: "b", ExpectedOutput: "ok"},
		{Input: "c", ExpectedOutput: "final output"},
	}
	res := evaluate(context.Background(), exec, "python", "code", "", cases, 1000, 128)

	if res.status != "accepted" {
		t.Fatalf("status = %q, want accepted", res.status)
	}
	if res.stdout != "final output" {
		t.Errorf("stdout = %q, want the last case's output %q", res.stdout, "final output")
	}
	if res.expectedOutput != "" {
		t.Errorf("expectedOutput = %q, want empty on accepted", res.expectedOutput)
	}
}

func TestEvaluate_SQLIgnoresRowOrderByDefault(t *testing.T) {
	exec := &scriptedExecutor{
		// Same rows as expected, reversed order.
		results: []RunResult{{Stdout: "2|y\n1|x"}},
		errs:    []error{nil},
	}
	cases := []testCaseRecord{{Input: "seed", ExpectedOutput: "1|x\n2|y", OrderMatters: false}}

	res := evaluate(context.Background(), exec, "sql", "SELECT * FROM t", "CREATE TABLE t(a,b)", cases, 1000, 128)

	if res.status != "accepted" {
		t.Errorf("status = %q, want accepted (row order shouldn't matter)", res.status)
	}
}

func TestEvaluate_SQLOrderMattersRejectsWrongOrder(t *testing.T) {
	exec := &scriptedExecutor{
		results: []RunResult{{Stdout: "2|y\n1|x"}},
		errs:    []error{nil},
	}
	cases := []testCaseRecord{{Input: "seed", ExpectedOutput: "1|x\n2|y", OrderMatters: true}}

	res := evaluate(context.Background(), exec, "sql", "SELECT * FROM t ORDER BY a", "CREATE TABLE t(a,b)", cases, 1000, 128)

	if res.status != "wrong_answer" {
		t.Errorf("status = %q, want wrong_answer (order_matters=true must compare exactly)", res.status)
	}
}

func TestEvaluate_SQLConcatenatesSchemaSeedAndQuery(t *testing.T) {
	var gotSource, gotInput string
	exec := &recordingExecutor{
		onRun: func(req RunRequest) (RunResult, error) {
			gotSource = req.SourceCode
			gotInput = req.Input
			return RunResult{Stdout: "ok"}, nil
		},
	}
	cases := []testCaseRecord{{Input: "INSERT INTO t VALUES (1)", ExpectedOutput: "ok"}}

	evaluate(context.Background(), exec, "sql", "SELECT 'ok'", "CREATE TABLE t(a)", cases, 1000, 128)

	wantSource := "CREATE TABLE t(a)\n\nINSERT INTO t VALUES (1)\n\nSELECT 'ok'"
	if gotSource != wantSource {
		t.Errorf("SourceCode = %q, want %q", gotSource, wantSource)
	}
	if gotInput != "" {
		t.Errorf("Input = %q, want empty (sqlite3 reads the script, not stdin)", gotInput)
	}
}

// recordingExecutor calls onRun for every RunRequest so a test can inspect
// exactly what evaluate builds, rather than only replaying canned results.
type recordingExecutor struct {
	onRun func(RunRequest) (RunResult, error)
}

func (e *recordingExecutor) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	return e.onRun(req)
}
