## Context

The root `go.mod` already depends on `github.com/golang-migrate/migrate/v4` (with `lib/pq` as the indirect PostgreSQL driver). The `migrations/` directory at the repo root holds versioned SQL files (`000001_initial_schema.up.sql`, `000001_initial_schema.down.sql`). No CLI entry point exists yet to drive those files. The Makefile's `make migrate` target already invokes `go run ./cmd/migrate up`, so the binary path is fixed.

## Goals / Non-Goals

**Goals:**
- Implement `cmd/migrate/main.go` in the root module (`github.com/caze/ascend`)
- Support `up`, `down`, and `version` subcommands
- Read `DATABASE_URL` from env; fail fast with a clear error if absent
- Use the `migrations/` directory as the migration source (relative path from CWD)
- Print human-readable output; exit non-zero on any error

**Non-Goals:**
- Interactive prompts or confirmation steps
- Embedding migrations into the binary (not needed for a dev/ops tool run from the repo root)
- Support for migration sources other than the local `migrations/` directory
- A `create` subcommand for generating new migration files

## Decisions

### Use `golang-migrate` directly (no custom runner)
`golang-migrate` is already in `go.mod`, handles idempotency tracking via a `schema_migrations` table, and supports `up`/`down`/`version` natively. Writing a custom runner would duplicate solved work.

*Alternative considered*: raw `database/sql` + manual SQL execution — rejected: no version tracking, no idempotency, more code.

### Migration source: `file://migrations` (relative path)
The tool is always invoked as `go run ./cmd/migrate <cmd>` from the repo root (both by developers and by the Makefile). Using a relative `file://migrations` path is the simplest approach that works in all expected contexts.

*Alternative considered*: `//go:embed migrations` — Go's embed cannot reference a path above the package directory, and `cmd/migrate` is a sibling of `migrations/`, not a parent. Would require copying SQL files into `cmd/migrate/migrations/`, creating duplication.

*Alternative considered*: `MIGRATIONS_DIR` env var — adds flexibility that isn't needed; defaults and overrides belong in CI scripts, not in the binary's interface.

### Subcommand parsing: `os.Args[1]` + `flag` package
Three subcommands (`up`, `down`, `version`) with one optional flag (`-steps` for `down`) don't warrant a CLI library. `os.Args` + stdlib `flag` keeps the dependency footprint minimal.

*Alternative considered*: `cobra` or `urfave/cli` — rejected: overkill for three subcommands; adds a new dependency.

### PostgreSQL driver: `lib/pq`
Already an indirect dependency in `go.mod`. `golang-migrate`'s postgres driver wraps `lib/pq`. No additional dependency needed.

## Risks / Trade-offs

- **CWD sensitivity**: `file://migrations` breaks if the binary is invoked from a directory other than the repo root. → Mitigation: document in `--help` output; the Makefile enforces the correct CWD.
- **`down` data loss**: Running `down` in production reverts schema changes and may delete data. → Mitigation: no mitigation in the tool itself (out of scope); documented in spec as caller responsibility.
- **No dry-run mode**: Users cannot preview which migrations will run. → Acceptable for now; `version` shows current state.
