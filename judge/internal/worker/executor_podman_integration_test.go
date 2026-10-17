//go:build integration

package worker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Opt-in: exclusively the caller's private rootless Podman; never Docker.
func TestExecutorPodman(t *testing.T) {
	if os.Getenv("ASCEND_TEST_PODMAN") != "1" {
		t.Skip("set ASCEND_TEST_PODMAN=1 for private rootless sandbox tests")
	}
	info, err := exec.Command("podman", "info", "--format", "{{.Host.Security.Rootless}}").Output()
	if err != nil || strings.TrimSpace(string(info)) != "true" {
		t.Fatal("private rootless Podman required")
	}
	containers := func() string {
		t.Helper()
		data, err := exec.Command("podman", "ps", "-a", "--filter", "name=ascend-run-", "--format", "{{.Names}}").Output()
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	before := containers()
	for _, tc := range []struct {
		name, source, input, want string
		failure                   error
	}{
		{"stdin", "print(input().upper())", "hello\n", "HELLO\n", nil},
		{"runtime", "raise ValueError('expected fixture error')", "", "", ErrRuntimeError},
		{"output", "print('x'*100000)", "", "", ErrOutputLimitExceeded},
		{"timeout", "while True: pass", "", "", ErrTimeLimitExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			timeout := 15 * time.Second
			if tc.name == "timeout" {
				timeout = 3 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			result, err := NewDockerExecutor("podman").Run(ctx, RunRequest{Language: LanguagePython, SourceCode: tc.source, Input: tc.input, MemoryLimitMB: 64})
			if tc.failure == nil && err != nil || tc.failure != nil && !errors.Is(err, tc.failure) {
				t.Fatalf("error = %v, stderr = %q", err, result.Stderr)
			}
			if tc.failure == nil && result.Stdout != tc.want {
				t.Fatalf("stdout = %q", result.Stdout)
			}
			if after := containers(); after != before {
				t.Fatalf("sandbox containers remain: before %q after %q", before, after)
			}
		})
	}
}
