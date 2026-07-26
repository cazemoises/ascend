package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv))
}

func run(args []string, getenv func(string) string) int {
	if len(args) == 0 {
		usage()
		return 1
	}

	cmd, rest := args[0], args[1:]

	switch cmd {
	case "up", "down", "version":
	default:
		usage()
		return 1
	}

	dbURL := getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "error: DATABASE_URL is not set")
		return 1
	}

	switch cmd {
	case "up":
		return cmdUp(dbURL)
	case "down":
		return cmdDown(dbURL, rest)
	case "version":
		return cmdVersion(dbURL)
	}
	return 0
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: migrate <command> [options]")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  up              apply all pending migrations")
	fmt.Fprintln(os.Stderr, "  down [-steps N] revert last N migrations (default 1)")
	fmt.Fprintln(os.Stderr, "  version         print current schema version")
}

func newMigrate(dbURL string) (*migrate.Migrate, error) {
	return migrate.New("file://migrations", dbURL)
}

func cmdUp(dbURL string) int {
	m, err := newMigrate(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			fmt.Println("database is up to date")
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Println("migrations applied")
	return 0
}

func cmdDown(dbURL string, args []string) int {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	steps := fs.Int("steps", 1, "number of migrations to revert")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	m, err := newMigrate(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer m.Close()

	if err := m.Steps(-*steps); err != nil {
		if err == migrate.ErrNoChange {
			fmt.Println("nothing to revert")
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("reverted %d migration(s)\n", *steps)
	return 0
}

func cmdVersion(dbURL string) int {
	m, err := newMigrate(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		if err == migrate.ErrNilVersion {
			fmt.Println("no migrations applied")
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("version: %d, dirty: %v\n", version, dirty)
	return 0
}
