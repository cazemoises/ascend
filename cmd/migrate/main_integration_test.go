//go:build integration

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// repoRoot returns the absolute path to the repository root, regardless of
// which directory the test binary runs from.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is …/cmd/migrate/main_integration_test.go; go up two levels
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func integrationEnv(t *testing.T) func(string) string {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	return func(key string) string {
		if key == "DATABASE_URL" {
			return dbURL
		}
		return ""
	}
}

func TestIntegration_Up(t *testing.T) {
	env := integrationEnv(t)
	if err := os.Chdir(repoRoot(t)); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	code := run([]string{"up"}, env)
	if code != 0 {
		t.Errorf("up: expected exit 0, got %d", code)
	}
}

func TestIntegration_Version(t *testing.T) {
	env := integrationEnv(t)
	if err := os.Chdir(repoRoot(t)); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	code := run([]string{"version"}, env)
	if code != 0 {
		t.Errorf("version: expected exit 0, got %d", code)
	}
}

func TestIntegration_Down(t *testing.T) {
	env := integrationEnv(t)
	if err := os.Chdir(repoRoot(t)); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	code := run([]string{"down", "-steps", "1"}, env)
	if code != 0 {
		t.Errorf("down -steps 1: expected exit 0, got %d", code)
	}
}
