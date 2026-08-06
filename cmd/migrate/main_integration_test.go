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

// TestIntegration_Down exercises `down` against whatever database
// DATABASE_URL points at, which — unlike a throwaway test schema — may be a
// real dev database other tests and manual sessions share. A DROP COLUMN
// dropped by `down` here would otherwise sit reverted for every test (or
// person) that runs afterward, silently discarding that column's data. This
// test's own job is only to confirm `down` works, not to leave the schema
// downgraded, so it re-applies `up` in cleanup — even on failure — to leave
// the database exactly as it found it.
func TestIntegration_Down(t *testing.T) {
	env := integrationEnv(t)
	if err := os.Chdir(repoRoot(t)); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if code := run([]string{"up"}, env); code != 0 {
			t.Errorf("cleanup: re-applying migrations after down failed, exit %d", code)
		}
	})
	code := run([]string{"down", "-steps", "1"}, env)
	if code != 0 {
		t.Errorf("down -steps 1: expected exit 0, got %d", code)
	}
}
