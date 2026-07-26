## 1. Test Setup

- [x] 1.1 Create `cmd/migrate/main_test.go` with table-driven tests for subcommand parsing (unknown command, no command → usage + non-zero exit)
- [x] 1.2 Write integration test skeleton for `up` / `down` / `version` (requires a real Postgres; mark with `//go:build integration` build tag)

## 2. Core Implementation

- [x] 2.1 Create `cmd/migrate/main.go` with `main()` that reads `os.Args[1]` and dispatches to subcommand handlers
- [x] 2.2 Implement `DATABASE_URL` env var reading with fast-fail (stderr message + `os.Exit(1)`) if absent
- [x] 2.3 Implement `up` handler: open `golang-migrate` instance with `file://migrations` source and postgres driver, call `m.Up()`, handle `migrate.ErrNoChange`
- [x] 2.4 Implement `down` handler: parse optional `-steps N` flag (default 1), call `m.Steps(-N)`
- [x] 2.5 Implement `version` handler: call `m.Version()`, handle `migrate.ErrNilVersion` (no migrations applied)
- [x] 2.6 Implement `usage()` helper that prints valid subcommands to stderr; call it on unknown/missing subcommand and exit non-zero

## 3. Verification

- [x] 3.1 Run `go build ./cmd/migrate` from repo root — ensure it compiles with no errors
- [x] 3.2 Run unit tests: `go test ./cmd/migrate/...`
- [x] 3.3 Smoke test against a live Postgres: `make dev` (docker compose up), then `go run ./cmd/migrate version`, `up`, `version`, `down`, `version`
- [x] 3.4 Confirm `make migrate` (`go run ./cmd/migrate up`) succeeds end-to-end in the Docker Compose stack
