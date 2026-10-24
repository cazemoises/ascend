package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"encoding/json"
	"github.com/caze/ascend/pkg/httplab"
)

// The client runs in a separate, root-owned exec process. Student processes
// remain uid 65534 and cannot write the client's stdout or replace its code.
// No ports are published: both processes use the sandbox's isolated loopback.
const httpClient = `import http.client,json,sys,time
requests=json.load(sys.stdin)
for attempt in range(300):
 try:
  c=http.client.HTTPConnection('127.0.0.1',8000,timeout=2)
  c.connect()
  c.close()
  break
 except OSError:
  time.sleep(.02)
else:
 raise RuntimeError('FastAPI server did not start')
responses=[]
for r in requests:
 body=json.dumps(r['json']).encode() if 'json' in r else None
 c=http.client.HTTPConnection('127.0.0.1',8000,timeout=2)
 c.request(r['method'],r['path'],body,{'Content-Type':'application/json'} if body is not None else {})
 rsp=c.getresponse()
 b=rsp.read(16385)
 if len(b)>16384: raise RuntimeError('HTTP response exceeds 16 KiB')
 try: value=json.loads(b) if b else None
 except ValueError:
  responses.append({'status':rsp.status,'json':None,'raw_body':b.decode('utf-8',errors='replace')})
  c.close()
  continue
 responses.append({'status':rsp.status,'json':value})
 c.close()
print(json.dumps(responses,ensure_ascii=True))
`

func (e *dockerCLIExecutor) runHTTP(ctx context.Context, name string, payload []byte, input string) (RunResult, error) {
	var requests []json.RawMessage
	if err := json.Unmarshal([]byte(input), &requests); err != nil || len(requests) < 1 || len(requests) > 30 {
		return RunResult{}, fmt.Errorf("invalid HTTP request batch")
	}
	for _, raw := range requests {
		if _, err := httplab.ParseRequest(string(raw)); err != nil {
			return RunResult{}, err
		}
	}
	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	logs := &boundedOutput{cancel: cancel}
	server := exec.CommandContext(runCtx, e.binary, "start", "--attach", "--interactive", name)
	server.Stdin = bytes.NewReader(payload)
	server.Stdout = outputStream{output: logs, stderr: true}
	server.Stderr = outputStream{output: logs, stderr: true}
	server.WaitDelay = time.Second
	done := make(chan error, 1)
	go func() { done <- server.Run() }()
	var client *exec.Cmd
	var out *boundedOutput
	// The CLI may need a moment to change the container from created to running.
	var err error
	for attempt := 0; attempt < 100; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				break
			case <-time.After(20 * time.Millisecond):
			}
		}
		client = exec.CommandContext(runCtx, e.binary, "exec", "--interactive", "--user", "0:0", name, "python", "-I", "-c", httpClient)
		client.Stdin = bytes.NewBufferString(input)
		out = &boundedOutput{cancel: cancel}
		client.Stdout = outputStream{output: out}
		client.Stderr = outputStream{output: out, stderr: true}
		client.WaitDelay = time.Second
		err = client.Run()
		if err == nil || ctx.Err() != nil || !(strings.Contains(out.stderr.String(), "not running") || strings.Contains(out.stderr.String(), "state improper")) {
			break
		}
	}
	cancel(nil)
	<-done
	res := RunResult{Stdout: out.stdout.String(), Stderr: logs.stderr.String() + out.stderr.String()}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return res, ErrTimeLimitExceeded
	}
	if errors.Is(context.Cause(runCtx), ErrOutputLimitExceeded) {
		res.Stderr = "Output limit exceeded (64 KiB server logs or HTTP responses)."
		return res, ErrOutputLimitExceeded
	}
	if err != nil {
		return res, fmt.Errorf("HTTP client: %w", errors.Join(ErrRuntimeError, err))
	}
	var responses []json.RawMessage
	if err := json.Unmarshal([]byte(res.Stdout), &responses); err != nil || len(responses) != len(requests) {
		return res, fmt.Errorf("invalid HTTP response batch")
	}
	return res, nil
}

// A batch shares one server and one deadline, allowing POST/GET stateful labs.
// Only requests enter the sandbox; expected responses and verdicts remain here.
func evaluateHTTP(ctx context.Context, executor DockerExecutor, source string, cases []testCaseRecord, timeout, memory int) evalResult {
	res := evalResult{status: "runtime_error", total: len(cases)}
	// A batch runs every request before comparison. Even a sample failure's
	// server logs can contain later hidden requests, so suppress batch logs.
	hasHidden := false
	for _, tc := range cases {
		hasHidden = hasHidden || !tc.IsSample
	}
	if len(cases) == 0 || len(cases) > 30 {
		res.stderr = "HTTP lab requires 1–30 tests"
		return res
	}
	requests := make([]json.RawMessage, 0, len(cases))
	for _, tc := range cases {
		if _, err := httplab.ParseRequest(tc.Input); err != nil {
			res.stderr = err.Error()
			return res
		}
		requests = append(requests, json.RawMessage(tc.Input))
	}
	input, _ := json.Marshal(requests)
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	start := time.Now()
	result, err := executor.Run(runCtx, RunRequest{Language: LanguageFastAPI, SourceCode: source, Input: string(input), MemoryLimitMB: memory})
	res.execTimeMs = int(time.Since(start).Milliseconds())
	if !hasHidden {
		res.stderr = result.Stderr
	}
	if err != nil {
		if errors.Is(err, ErrTimeLimitExceeded) {
			res.status = "time_limit_exceeded"
		}
		if res.stderr == "" {
			res.stderr = err.Error()
		}
		return res
	}
	var responses []json.RawMessage
	if err := json.Unmarshal([]byte(result.Stdout), &responses); err != nil || len(responses) != len(cases) {
		res.stderr = "Invalid HTTP response batch"
		return res
	}
	res.status = "accepted"
	exploratory := true
	for i, tc := range cases {
		actual := string(responses[i])
		res.stdout = ""
		if tc.IsSample {
			res.stdout = actual
		}
		if _, err := httplab.ParseResponse(actual); err != nil {
			res.status = "runtime_error"
			res.stderr = err.Error()
			return res
		}
		if tc.ExpectedOutput != "" {
			exploratory = false
			if !httplab.Match(actual, tc.ExpectedOutput) {
				res.status = "wrong_answer"
				res.failedIsSample = tc.IsSample
				if tc.IsSample {
					res.expectedOutput = tc.ExpectedOutput
					res.failedInput = tc.Input
				}
				return res
			}
		}
		res.passedCount++
	}
	if exploratory {
		res.stdout = result.Stdout
	}
	return res
}
