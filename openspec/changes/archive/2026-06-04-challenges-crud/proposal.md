## Why

The Ascend platform has a `challenges` table and a stubbed `/api/v1/` router but no HTTP interface to create, retrieve, or delete challenges. Without these endpoints, the judge cannot be linked to problems and the frontend has nothing to display. The table schema is also missing execution constraints (`time_limit_ms`, `memory_limit_mb`) that the judge will need.

## What Changes

- Add `time_limit_ms` (integer, ms) and `memory_limit_mb` (integer, MB) columns to the `challenges` table via a new migration
- Implement six REST endpoints under `/api/v1/`:
  - `GET  /api/v1/challenges` — list all challenges (paginated, sorted by created_at desc)
  - `POST /api/v1/challenges` — create a challenge, returns 201 + created resource
  - `GET  /api/v1/challenges/{id}` — get one challenge by UUID
  - `DELETE /api/v1/challenges/{id}` — delete a challenge and its test cases, returns 204
  - `POST /api/v1/challenges/{id}/test-cases` — append a test case to a challenge, returns 201
  - `GET  /api/v1/challenges/{id}/test-cases` — list test cases for a challenge, ordered by ordinal
- Wire all handlers into the existing chi router at `/api/v1/`
- Connect the API to PostgreSQL via `DATABASE_URL`

## Capabilities

### New Capabilities

- `challenges-crud`: REST CRUD endpoints for challenges and their test cases, backed by PostgreSQL

### Modified Capabilities

- `database-schema`: challenges table gains `time_limit_ms INTEGER NOT NULL DEFAULT 2000` and `memory_limit_mb INTEGER NOT NULL DEFAULT 256`; the spec requirement must reflect these new mandatory columns

## Impact

- New migration: `migrations/000002_challenges_limits.up.sql` + `.down.sql`
- New packages: `api/internal/handler/challenges.go`, `api/internal/store/challenges.go` (or equivalent)
- `api/internal/router/router.go` updated to mount challenge routes
- Root `go.mod` gains `github.com/lib/pq` as a direct dependency (currently indirect via migrate)
- No changes to `judge/`, `web/`, or `cmd/`
