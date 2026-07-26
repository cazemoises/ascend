package main

import (
	"testing"
)

func TestRun_NoArgs(t *testing.T) {
	code := run([]string{}, func(string) string { return "" })
	if code == 0 {
		t.Error("expected non-zero exit code when no args provided, got 0")
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"single unknown", []string{"foo"}},
		{"migrate-prefixed", []string{"migrate"}},
		{"help flag", []string{"--help"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := run(tt.args, func(string) string { return "" })
			if code == 0 {
				t.Errorf("args %v: expected non-zero exit code, got 0", tt.args)
			}
		})
	}
}

func TestRun_MissingDatabaseURL(t *testing.T) {
	noenv := func(string) string { return "" }
	for _, cmd := range []string{"up", "down", "version"} {
		t.Run(cmd, func(t *testing.T) {
			code := run([]string{cmd}, noenv)
			if code == 0 {
				t.Errorf("command %q: expected non-zero exit code when DATABASE_URL absent, got 0", cmd)
			}
		})
	}
}

func TestRun_DownStepsFlag_InvalidValue(t *testing.T) {
	env := func(key string) string {
		if key == "DATABASE_URL" {
			return "postgres://dummy"
		}
		return ""
	}
	// -steps with a non-integer should return non-zero (flag parse error)
	code := run([]string{"down", "-steps", "abc"}, env)
	if code == 0 {
		t.Error("expected non-zero exit code for invalid -steps value, got 0")
	}
}
