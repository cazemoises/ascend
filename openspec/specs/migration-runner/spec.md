# Spec: Migration Runner

## Purpose

The migration runner is a CLI tool (`cmd/migrate`) that applies, reverts, and reports the version of PostgreSQL database migrations. It wraps `golang-migrate` and reads the database connection from `DATABASE_URL`. It is the implementation behind `go run ./cmd/migrate <command>` and the Makefile `make migrate` target.

## Requirements

### Requirement: Runner reads DATABASE_URL and fails fast if absent
The migration runner SHALL read the database connection string from the `DATABASE_URL` environment variable. If `DATABASE_URL` is unset or empty, the runner SHALL print an error message to stderr and exit with a non-zero status code without attempting any database operation.

#### Scenario: DATABASE_URL is set
- **WHEN** `DATABASE_URL` is set to a valid PostgreSQL connection string
- **THEN** the runner proceeds to execute the requested subcommand

#### Scenario: DATABASE_URL is absent
- **WHEN** `DATABASE_URL` is not set in the environment
- **THEN** the runner prints an error to stderr containing "DATABASE_URL" and exits with a non-zero status code

### Requirement: `up` subcommand applies all pending migrations
Running `go run ./cmd/migrate up` SHALL apply all unapplied migration files from the `migrations/` directory in ascending numeric order. If there are no pending migrations, the runner SHALL exit with code 0 and print a message indicating nothing to do.

#### Scenario: Pending migrations exist
- **WHEN** `go run ./cmd/migrate up` is run and there are unapplied migrations
- **THEN** all pending migrations are applied in order and the runner exits with code 0

#### Scenario: No pending migrations
- **WHEN** `go run ./cmd/migrate up` is run and all migrations are already applied
- **THEN** the runner exits with code 0 and prints a message indicating the database is up to date

#### Scenario: Migration file error
- **WHEN** a migration file contains invalid SQL
- **THEN** the runner prints the error to stderr and exits with a non-zero status code

### Requirement: `down` subcommand reverts the last N migrations
Running `go run ./cmd/migrate down` SHALL revert the most recently applied migration. An optional `-steps N` flag SHALL allow reverting N migrations in descending order. The default for N is 1.

#### Scenario: Revert one migration (default)
- **WHEN** `go run ./cmd/migrate down` is run without flags and at least one migration is applied
- **THEN** the most recently applied migration is reverted and the runner exits with code 0

#### Scenario: Revert multiple migrations with -steps
- **WHEN** `go run ./cmd/migrate down -steps 3` is run and at least 3 migrations are applied
- **THEN** the last 3 applied migrations are reverted in descending order and the runner exits with code 0

#### Scenario: No migrations to revert
- **WHEN** `go run ./cmd/migrate down` is run and no migrations have been applied
- **THEN** the runner exits with code 0 and prints a message indicating nothing to revert

### Requirement: `version` subcommand prints the current schema version
Running `go run ./cmd/migrate version` SHALL print the version number (migration file number) of the most recently applied migration and its dirty state. If no migrations have been applied, it SHALL print a message indicating the database has no applied migrations.

#### Scenario: Migrations have been applied
- **WHEN** `go run ./cmd/migrate version` is run after applying migrations
- **THEN** the runner prints the current version number and dirty state (e.g., `version: 1, dirty: false`) and exits with code 0

#### Scenario: No migrations applied
- **WHEN** `go run ./cmd/migrate version` is run against a fresh database
- **THEN** the runner prints a message indicating no migrations have been applied and exits with code 0

### Requirement: Unknown subcommand prints usage and exits non-zero
If the runner is invoked with an unrecognized subcommand or no subcommand, it SHALL print a usage message listing valid subcommands and exit with a non-zero status code.

#### Scenario: Unknown subcommand
- **WHEN** `go run ./cmd/migrate foo` is run
- **THEN** the runner prints a usage message to stderr and exits with a non-zero status code

#### Scenario: No subcommand
- **WHEN** `go run ./cmd/migrate` is run without arguments
- **THEN** the runner prints a usage message to stderr and exits with a non-zero status code
