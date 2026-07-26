## 1. Migration

- [x] 1.1 Create `migrations/000002_challenges_limits.up.sql`: `ALTER TABLE challenges ADD COLUMN time_limit_ms INTEGER NOT NULL DEFAULT 2000, ADD COLUMN memory_limit_mb INTEGER NOT NULL DEFAULT 256`
- [x] 1.2 Create `migrations/000002_challenges_limits.down.sql`: `ALTER TABLE challenges DROP COLUMN time_limit_ms, DROP COLUMN memory_limit_mb`
- [x] 1.3 Apply migration: run `go run ./cmd/migrate up` (or via Docker) and verify columns exist

## 2. Store Layer

- [x] 2.1 Add `github.com/lib/pq` to `api/go.mod` (`go get github.com/lib/pq` in `api/`)
- [x] 2.2 Create `api/internal/store/store.go`: `Store` struct wrapping `*sql.DB`, `New(db *sql.DB) *Store` constructor
- [x] 2.3 Implement `Store.ListChallenges(ctx, limit, offset int) ([]Challenge, error)` — `SELECT` ordered by `created_at DESC`
- [x] 2.4 Implement `Store.CreateChallenge(ctx, req CreateChallengeRequest) (Challenge, error)` — `INSERT … RETURNING *`; return typed error for unique violation (slug conflict) and FK/enum violation
- [x] 2.5 Implement `Store.GetChallenge(ctx, id string) (Challenge, error)` — `SELECT … WHERE id = $1`; return `ErrNotFound` sentinel when no rows
- [x] 2.6 Implement `Store.DeleteChallenge(ctx, id string) error` — delete test cases first, then challenge; return `ErrNotFound` if challenge missing, typed error for FK violation (submissions reference)
- [x] 2.7 Implement `Store.CreateTestCase(ctx, challengeID string, req CreateTestCaseRequest) (TestCase, error)` — compute `ordinal` as `COALESCE(MAX(ordinal),-1)+1` in same transaction; return `ErrNotFound` if challenge missing
- [x] 2.8 Implement `Store.ListTestCases(ctx, challengeID string) ([]TestCase, error)` — `SELECT … WHERE challenge_id = $1 ORDER BY ordinal ASC`; return `ErrNotFound` if challenge doesn't exist

## 3. Handler Layer

- [x] 3.1 Create `api/internal/handler/challenges.go`: define `ChallengesHandler` struct with `*store.Store`, implement `Routes(r chi.Router)` mounting all six endpoints
- [x] 3.2 Implement `GET /challenges` handler: parse `limit`/`offset` query params, call `store.ListChallenges`, write JSON 200
- [x] 3.3 Implement `POST /challenges` handler: decode JSON body, validate required fields and difficulty enum, call `store.CreateChallenge`, write JSON 201; map slug-conflict → 409, validation error → 422, bad JSON → 400
- [x] 3.4 Implement `GET /challenges/{id}` handler: call `store.GetChallenge`, write JSON 200; map `ErrNotFound` → 404
- [x] 3.5 Implement `DELETE /challenges/{id}` handler: call `store.DeleteChallenge`, write 204; map `ErrNotFound` → 404, FK violation → 409
- [x] 3.6 Implement `POST /challenges/{id}/test-cases` handler: decode JSON body, validate `expected_output`, call `store.CreateTestCase`, write JSON 201; map `ErrNotFound` → 404, validation → 422
- [x] 3.7 Implement `GET /challenges/{id}/test-cases` handler: call `store.ListTestCases`, write JSON 200; map `ErrNotFound` → 404

## 4. Wire Up

- [x] 4.1 Update `api/cmd/server/main.go`: read `DATABASE_URL`, open `*sql.DB` with `lib/pq`, ping on startup (fail-fast), pass to `router.New()`
- [x] 4.2 Update `router.New()` signature to accept `*store.Store`; mount `ChallengesHandler.Routes` under the `/api/v1/` group
- [x] 4.3 Add `_ "github.com/lib/pq"` blank import in the file that calls `sql.Open`

## 5. Tests

- [x] 5.1 Write table-driven unit tests for request validation in `api/internal/handler/challenges_test.go` (missing fields, invalid difficulty, bad JSON) using `httptest`
- [x] 5.2 Write store tests in `api/internal/store/store_test.go` using `//go:build integration` tag and a real Postgres; cover: create, get, list, delete (happy path + not-found), test-case creation + ordinal sequencing

## 6. Verification

- [x] 6.1 `go vet ./...` from `api/` — no errors
- [x] 6.2 `go test ./...` from `api/` — unit tests pass
- [x] 6.3 Rebuild Docker image (`docker compose build api`) and restart (`docker compose up -d api`)
- [x] 6.4 Smoke test: `POST /api/v1/challenges` → 201, `GET /api/v1/challenges` → 200 with created item, `GET /api/v1/challenges/{id}` → 200, `POST /api/v1/challenges/{id}/test-cases` → 201, `GET /api/v1/challenges/{id}/test-cases` → 200, `DELETE /api/v1/challenges/{id}` → 204
