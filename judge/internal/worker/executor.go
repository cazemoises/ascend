package worker

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
)

var (
	ErrTimeLimitExceeded = errors.New("time limit exceeded")
	ErrRuntimeError      = errors.New("runtime error")
)

type Language string

const (
	LanguagePython     Language = "python"
	LanguageJavaScript Language = "javascript"
	LanguageGo         Language = "go"
	LanguageSQL        Language = "sql"
	LanguageJava       Language = "java"
	LanguageC          Language = "c"
	LanguageCPP        Language = "cpp"
)

type RunRequest struct {
	Language      Language
	SourceCode    string
	Input         string
	MemoryLimitMB int
}

type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type DockerExecutor interface {
	Run(ctx context.Context, req RunRequest) (RunResult, error)
}

type dockerCLIExecutor struct {
	binary string
}

func NewDockerExecutor(binary string) DockerExecutor {
	if binary == "" {
		binary = "docker"
	}
	return &dockerCLIExecutor{binary: binary}
}

func (e *dockerCLIExecutor) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	image, command, fileName, err := containerConfig(req.Language)
	if err != nil {
		return RunResult{}, fmt.Errorf("container config: %w", err)
	}

	payload, err := buildPayloadArchive(fileName, req.SourceCode, req.Input)
	if err != nil {
		return RunResult{}, fmt.Errorf("build payload archive: %w", err)
	}

	args := []string{
		"run",
		"--rm",
		"-i",
		// Hardening: total network isolation plus least-privilege execution.
		// The submission runs as the unprivileged "nobody" user with every
		// capability dropped, no privilege escalation and a bounded process
		// count, so scripts cannot reach the network, escalate, or fork-bomb.
		"--network", "none",
		"--user", "65534:65534",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--pids-limit", "128",
		// Toolchains need a writable HOME/cache as nobody; keep it in /tmp,
		// which is the only world-writable path in the container.
		"-e", "HOME=/tmp",
		"-e", "GOCACHE=/tmp/.cache/go-build",
		"-e", "GOPATH=/tmp/go",
		"--memory", strconv.Itoa(req.MemoryLimitMB) + "m",
		"--cpus", "0.5",
		image,
	}
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, e.binary, args...)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	result := RunResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err == nil {
		return result, nil
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return result, fmt.Errorf("docker run timed out: %w", ErrTimeLimitExceeded)
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ProcessState != nil {
			result.ExitCode = exitErr.ProcessState.ExitCode()
		}
		return result, fmt.Errorf("docker run exit error: %w", ErrRuntimeError)
	}

	return result, fmt.Errorf("docker run: %w", ErrRuntimeError)
}

func containerConfig(language Language) (image string, command []string, fileName string, err error) {
	switch language {
	case LanguagePython:
		return "python:3.11-alpine", []string{"sh", "-c", "tar -xf - -C /tmp && python /tmp/solution.py < /tmp/input.txt"}, "solution.py", nil
	case LanguageJavaScript:
		return "node:20-alpine", []string{"sh", "-c", "tar -xf - -C /tmp && node /tmp/solution.js < /tmp/input.txt"}, "solution.js", nil
	case LanguageGo:
		return "golang:1.26-alpine", []string{"sh", "-c", "tar -xf - -C /tmp && cd /tmp && go run solution.go < input.txt"}, "solution.go", nil
	case LanguageSQL:
		// -list -separator '|' makes sqlite3's output deterministic: one row
		// per line, columns pipe-separated — the same format test_case rows
		// are stored in, so the judge can string-compare stdout directly.
		// solution.sql is the schema + seed + student query already
		// concatenated by the worker (see buildSQLScript); input.txt is
		// written but unused for this language.
		return "ascend-sqlite:latest",
			[]string{"sh", "-c", "tar -xf - -C /tmp && sqlite3 -list -separator '|' :memory: < /tmp/solution.sql"},
			"solution.sql", nil
	case LanguageJava:
		// eclipse-temurin:21-jdk-alpine (~550MB) is far larger than the other
		// language images (python:3.11-alpine is ~85MB) — a node that has
		// never run a Java submission will spend ~25s+ on the first `docker
		// run` just pulling it, which the per-test-case context.WithTimeout
		// in evaluate() (wrapping this entire command, pull included) will
		// read as a false time_limit_exceeded until the image is cached
		// locally. Known, accepted risk, same class as any uncached image,
		// just a bigger one here — not fixed by this change (that would be
		// separating pull time from the execution clock, a judge-wide
		// change, not a language-specific one). javac+JVM startup overhead
		// per invocation (recompiled from scratch every run, like Go's `go
		// run`) is also higher than the interpreted languages even once
		// warm, worth accounting for in time_limit_ms on Java challenges.
		// The public class must be named Solution to match the filename —
		// javac enforces this.
		return "eclipse-temurin:21-jdk-alpine",
			[]string{"sh", "-c", "tar -xf - -C /tmp && cd /tmp && javac Solution.java && java Solution < input.txt"},
			"Solution.java", nil
	case LanguageC:
		// ascend-cpp:latest (docker/cpp.Dockerfile) — no official minimal
		// Alpine image ships gcc/g++ pre-installed, same reasoning as
		// ascend-sqlite:latest next to it. A failed compile makes gcc exit
		// non-zero, so `&&` short-circuits before the run step and the
		// existing generic ExitError handling in Run() reports it as
		// ErrRuntimeError — the same verdict path every other language's
		// runtime failures already take, no new machinery needed. Same for
		// a submission that compiles but segfaults at runtime: the killed
		// process's exit code propagates through sh -c the same way, still
		// just an ErrRuntimeError return value, never a worker crash.
		return "ascend-cpp:latest",
			[]string{"sh", "-c", "tar -xf - -C /tmp && cd /tmp && gcc -O2 -std=c11 -o solution solution.c && ./solution < input.txt"},
			"solution.c", nil
	case LanguageCPP:
		return "ascend-cpp:latest",
			[]string{"sh", "-c", "tar -xf - -C /tmp && cd /tmp && g++ -O2 -std=c++17 -o solution solution.cpp && ./solution < input.txt"},
			"solution.cpp", nil
	default:
		return "", nil, "", fmt.Errorf("unsupported language: %s", language)
	}
}

func buildPayloadArchive(sourceFileName, sourceCode, input string) ([]byte, error) {
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)

	if err := writeTarFile(writer, sourceFileName, []byte(sourceCode)); err != nil {
		return nil, err
	}
	if err := writeTarFile(writer, "input.txt", []byte(input)); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("finalize tar archive: %w", err)
	}
	return buf.Bytes(), nil
}

func writeTarFile(writer *tar.Writer, name string, content []byte) error {
	head := &tar.Header{
		Name: name,
		Mode: 0o644,
		Size: int64(len(content)),
	}
	if err := writer.WriteHeader(head); err != nil {
		return fmt.Errorf("write tar header for %s: %w", name, err)
	}
	if _, err := writer.Write(content); err != nil {
		return fmt.Errorf("write tar content for %s: %w", name, err)
	}
	return nil
}