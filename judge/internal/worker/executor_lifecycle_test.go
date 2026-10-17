package worker

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The helper emulates only the container CLI, never executes student code.
func TestExecutorCLIHelper(t *testing.T) {
	if os.Getenv("ASCEND_CLI_HELPER") != "1" {
		return
	}
	var args []string
	for i, a := range os.Args {
		if a == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		os.Exit(90)
	}
	f, err := os.OpenFile(os.Getenv("ASCEND_CLI_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(91)
	}
	_, _ = f.WriteString(strings.Join(args, " ") + "\n")
	_ = f.Close()
	mode := os.Getenv("ASCEND_CLI_MODE")
	switch args[0] {
	case "create":
		if mode == "create_error" {
			os.Exit(125)
		}
		_, _ = io.WriteString(os.Stdout, "container-id\n")
	case "start":
		_, _ = io.Copy(io.Discard, os.Stdin)
		switch mode {
		case "timeout":
			time.Sleep(30 * time.Second)
		case "overflow":
			_, _ = io.WriteString(os.Stdout, strings.Repeat("x", maxRunOutput+1))
		case "stderr_overflow":
			_, _ = io.WriteString(os.Stderr, strings.Repeat("x", maxRunOutput+1))
		case "mixed_overflow":
			_, _ = io.WriteString(os.Stdout, strings.Repeat("x", maxRunOutput/2))
			_, _ = io.WriteString(os.Stderr, strings.Repeat("y", maxRunOutput/2+1))
		case "boundary":
			_, _ = io.WriteString(os.Stdout, strings.Repeat("x", maxRunOutput))
		case "runtime":
			_, _ = io.WriteString(os.Stderr, "student failure")
			os.Exit(7)
		default:
			_, _ = io.WriteString(os.Stdout, "ok\n")
		}
	case "kill":
		if mode == "kill_hang" {
			time.Sleep(30 * time.Second)
		}
		// An exited container may reject kill; cleanup must still remove it.
		os.Exit(1)
	case "rm":
		if mode == "cleanup_error" {
			os.Exit(1)
		}
	default:
		os.Exit(92)
	}
	os.Exit(0)
}

func TestExecutorLifecycle(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want error
	}{
		{"success", nil}, {"boundary", nil}, {"runtime", ErrRuntimeError},
		{"timeout", ErrTimeLimitExceeded}, {"overflow", ErrOutputLimitExceeded},
		{"stderr_overflow", ErrOutputLimitExceeded}, {"mixed_overflow", ErrOutputLimitExceeded},
		{"create_error", ErrRuntimeError}, {"cleanup_error", ErrRuntimeError}, {"kill_hang", nil},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(dir, "calls")
			binary := filepath.Join(dir, "cli")
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			script := "#!/bin/sh\nexec '" + strings.ReplaceAll(executable, "'", "'\\''") + "' -test.run=^TestExecutorCLIHelper$ -- \"$@\"\n"
			if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("ASCEND_CLI_HELPER", "1")
			t.Setenv("GORACE", "atexit_sleep_ms=0")
			t.Setenv("ASCEND_CLI_MODE", tc.mode)
			t.Setenv("ASCEND_CLI_LOG", log)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if tc.mode == "timeout" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
			}
			defer cancel()
			result, err := NewDockerExecutor(binary).Run(ctx, RunRequest{Language: LanguagePython, SourceCode: "print('ok')", MemoryLimitMB: 64})
			if tc.want == nil && err != nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("Run error = %v, want %v", err, tc.want)
			}
			if len(result.Stdout) > maxRunOutput || len(result.Stderr) > maxRunOutput {
				t.Fatal("unbounded output")
			}
			if tc.want == ErrOutputLimitExceeded && !strings.Contains(result.Stderr, "Output limit exceeded") {
				t.Fatal("missing actionable feedback")
			}
			if tc.mode == "runtime" && result.ExitCode != 7 {
				t.Fatalf("exit code = %d", result.ExitCode)
			}
			if tc.mode == "success" && result.Stdout != "ok\n" {
				t.Fatalf("stdout = %q", result.Stdout)
			}
			data, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			calls := strings.Split(strings.TrimSpace(string(data)), "\n")
			if !strings.HasPrefix(calls[0], "create --name ascend-run-") {
				t.Fatalf("create first: %s", data)
			}
			name := strings.Fields(calls[0])[2]
			if calls[len(calls)-1] != "rm -f "+name {
				t.Fatalf("independent cleanup missing: %s", data)
			}
			if tc.mode == "create_error" {
				if len(calls) != 3 {
					t.Fatalf("started after failed creation: %s", data)
				}
			} else if calls[1] != "start --attach --interactive "+name {
				t.Fatalf("start name mismatch: %s", data)
			}
			for _, flag := range []string{"--network none", "--user 65534:65534", "--cap-drop ALL", "--security-opt no-new-privileges", "--pids-limit 128", "--memory 64m", "--cpus 0.5"} {
				if !strings.Contains(calls[0], flag) {
					t.Fatalf("lost isolation %q", flag)
				}
			}
		})
	}
}
