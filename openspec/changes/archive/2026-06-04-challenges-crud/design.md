## Context

The `api/` module has a chi router with a stub `/api/v1/` route group and no database connection. The PostgreSQL `challenges` and `test_cases` tables exist (migration 000001), but `challenges` is missing `time_limit_ms` and `memory_limit_mb`. The API server binary reads no `DATABASE_URL` today. The `lib/pq` driver is an indirect dependency in root `go.mod` (brought in by golang-migrate); it becomes a direct dependency of the `api` module.

## Goals / Non-Goals

**Goals:**
- Connect the API server to PostgreSQL via `DATABASE_URL` env var
- Add `time_limit_ms` / `memory_limit_mb` columns via migration 000002
- Implement six endpoints: list, create, get, delete challenge; add, list test cases
- Keep the handler → store → DB layering consistent and testable

**Non-Goals:**
- Authentication / authorization (no auth layer yet)
- Pagination cursor style (simple limit/offset is sufficient for now)
- Full-text search or filtering on list endpoint
- Soft deletes (hard delete is simpler and sufficient)
- Caching layer

## Decisions

### Package structure: `api/internal/store` holds DB access
A `store.Store` struct wraps `*sql.DB` and exposes typed methods (`ListChallenges`, `CreateChallenge`, etc.). Handlers receive a `*store.Store` via closure. This keeps SQL out of handler files and makes the store swappable for tests.

*Alternative*: inline SQL in handlers — rejected: hard to test, mixes concerns.  
*Alternative*: sqlc code generation — rejected: adds a new toolchain dependency; overkill for six queries.

### DB access: `database/sql` + `lib/pq`
Stdlib-first approach, already consistent with the project conventions. `lib/pq` is already in `go.mod`. No new direct dependencies needed in the root module; `api/go.mod` gains `github.com/lib/pq`.

*Alternative*: `pgx` — more features (binary protocol, COPY), but a new dependency and more setup.

### DB connection: opened in `main()`, injected into router
`main.go` reads `DATABASE_URL`, calls `sql.Open`, pings on startup (fail-fast), and passes `*sql.DB` to `router.New()`. The router passes it to handlers. Same pattern as Redis in the judge worker.

### Error responses: `{"error": "<message>"}` JSON with appropriate status
All error paths return `Content-Type: application/json` and a JSON body. HTTP status codes:
- 400 Bad Request — malformed JSON body
- 404 Not Found — challenge/test-case not found
- 422 Unprocessable Entity — validation failure (missing required field, invalid enum)
- 500 Internal Server Error — unexpected DB error

### Pagination on list: `?limit` / `?offset` query params
Default `limit=50`, max `limit=100`. Simple and stateless; good enough before the dataset grows.

### Test-case ordinal: `MAX(ordinal)+1` assigned by server
The client does not control `ordinal`. The store computes the next ordinal atomically using `SELECT COALESCE(MAX(ordinal), -1) + 1` within the same transaction as the INSERT.

### Migration 000002: additive columns with defaults
`ALTER TABLE challenges ADD COLUMN time_limit_ms INTEGER NOT NULL DEFAULT 2000` and `ALTER TABLE challenges ADD COLUMN memory_limit_mb INTEGER NOT NULL DEFAULT 256`. Both are non-null with defaults so the migration is safe on a non-empty table and does not require a data backfill.

## Risks / Trade-offs

- **No auth**: any caller can delete any challenge. → Acceptable at this stage; auth is a future change.
- **Hard delete cascades**: deleting a challenge with submissions will fail if submissions have a FK to challenges with no `ON DELETE CASCADE`. → The submissions table has `REFERENCES challenges(id)` without cascade; delete will return a FK violation. Mitigation: `DELETE /challenges/{id}` returns 409 Conflict if submissions reference the challenge, documented in spec.
- **Limit/offset pagination**: inconsistent results if rows are inserted between pages. → Acceptable for admin-style CRUD; no strong ordering guarantees needed yet.
- **`lib/pq` driver in api module**: adds a direct dependency. → Low risk; it's already an indirect dep in the workspace.

## Migration Plan

1. Apply migration 000002 (`go run ./cmd/migrate up`) before or after deploying the new API binary — the columns have defaults, so the old binary is forward-compatible.
2. Rollback: `go run ./cmd/migrate down` removes the columns; no data loss beyond the column values.
