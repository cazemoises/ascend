## Why

The database schema is managed via migration files in `migrations/`, but there is no CLI tool to apply or revert them. The spec for `database-schema` already mandates `go run ./cmd/migrate up`, but `cmd/migrate` does not exist, making the project's database lifecycle unmanageable without a custom script or raw `psql`.

## What Changes

- Add `cmd/migrate/main.go` — a CLI entry point that wraps `golang-migrate` (already a root module dependency) to manage PostgreSQL migrations
- Support three subcommands: `up` (apply all pending), `down` (revert last N steps, default 1), `version` (print current applied version)
- Read database connection from `DATABASE_URL` environment variable; exit non-zero with a clear message if unset
- Print progress and errors to stdout/stderr in human-readable form

## Capabilities

### New Capabilities

- `migration-runner`: CLI tool (`cmd/migrate`) that applies, reverts, and reports the status of PostgreSQL migrations using `golang-migrate` and `DATABASE_URL`

### Modified Capabilities

<!-- None — database-schema spec requirements are unchanged; the runner is the missing implementation -->

## Impact

- New file: `cmd/migrate/main.go` (root module `github.com/caze/ascend`)
- Uses existing `github.com/golang-migrate/migrate/v4` dependency already in `go.mod`
- Uses existing `migrations/` directory as the migration source
- No changes to `api/`, `judge/`, or `web/`
- Makefile `make migrate` target already calls `go run ./cmd/migrate up` — this change makes that target functional
