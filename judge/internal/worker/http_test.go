package worker

import (
	"context"
	"testing"
)

func TestHTTPFeedbackVisibility(t *testing.T) {
	for _, tc := range []struct {
		name             string
		sample           bool
		expected, status string
	}{
		{"hidden failure", false, `{"status":200,"json":"expected-secret"}`, "wrong_answer"},
		{"hidden success", false, `{"status":200,"json":"actual-secret"}`, "accepted"},
		{"sample failure", true, `{"status":200,"json":"expected-secret"}`, "wrong_answer"},
		{"exploration", true, "", "accepted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := &scriptedExecutor{results: []RunResult{{Stdout: `[{"status":200,"json":"actual-secret"}]`, Stderr: "request-secret"}}, errs: []error{nil}}
			result := evaluateHTTP(context.Background(), executor, "code", []testCaseRecord{{Input: `{"method":"GET","path":"/request-secret"}`, ExpectedOutput: tc.expected, IsSample: tc.sample}}, 1000, 128)
			if result.status != tc.status {
				t.Fatalf("status=%s", result.status)
			}
			if !tc.sample && (result.stdout != "" || result.stderr != "" || result.failedInput != "" || result.expectedOutput != "") {
				t.Fatalf("hidden feedback leaked: %+v", result)
			}
			if tc.sample && (result.stdout == "" || result.stderr == "") {
				t.Fatalf("missing public feedback: %+v", result)
			}
		})
	}
}

func TestHTTPMixedVisibility(t *testing.T) {
	for _, tc := range []struct {
		name        string
		firstPublic bool
	}{{"public failure before hidden", true}, {"public failure after hidden", false}} {
		t.Run(tc.name, func(t *testing.T) {
			good := testCaseRecord{Input: `{"method":"GET","path":"/secret"}`, ExpectedOutput: `{"status":200,"json":1}`, IsSample: false}
			bad := testCaseRecord{Input: `{"method":"GET","path":"/public"}`, ExpectedOutput: `{"status":200,"json":2}`, IsSample: true}
			cases := []testCaseRecord{good, bad}
			if tc.firstPublic {
				cases = []testCaseRecord{bad, good}
			}
			executor := &scriptedExecutor{results: []RunResult{{Stdout: `[{"status":200,"json":1},{"status":200,"json":1}]`, Stderr: "secret later request"}}, errs: []error{nil}}
			result := evaluateHTTP(context.Background(), executor, "code", cases, 1000, 128)
			if result.status != "wrong_answer" || result.stderr != "" {
				t.Fatalf("invalid mixed feedback %+v", result)
			}
			if result.failedIsSample != tc.firstPublic || (result.expectedOutput != "") != tc.firstPublic || (result.stdout != "") != tc.firstPublic {
				t.Fatalf("state-tainted feedback: %+v", result)
			}
		})
	}
}
