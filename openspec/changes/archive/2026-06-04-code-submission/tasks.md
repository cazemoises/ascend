## 1. Database Migration

- [x] 1.1 Write `migrations/000003_submissions.up.sql` creating the `submissions` table
- [x] 1.2 Write `migrations/000003_submissions.down.sql` dropping the `submissions` table
- [x] 1.3 Apply migration via Docker and verify schema with `\d submissions`

## 2. Store Layer — RED (write failing tests first)

- [x] 2.1 Write `api/internal/store/submission_test.go` with unit/integration tests: `CreateSubmission` happy path, challenge not found → `ErrNotFound`
- [x] 2.2 Verify tests fail to compile or fail at runtime (RED)

## 3. Store Layer — GREEN

- [x] 3.1 Add `Submission` struct and `CreateSubmissionRequest` to `api/internal/store/store.go`
- [x] 3.2 Implement `(s *Store) CreateSubmission(ctx, req) (Submission, error)` — INSERT + map FK violation to `ErrNotFound`
- [x] 3.3 Add Redis client field to `Store` struct; update `store.New` to accept `*redis.Client`
- [x] 3.4 Implement Redis LPUSH in `CreateSubmission` after INSERT (enqueue `{submission_id, challenge_id}` JSON)
- [x] 3.5 Verify store tests pass (GREEN)

## 4. Handler — RED

- [x] 4.1 Write handler unit tests in `api/internal/handler/submissions_test.go`: bad JSON → 400, missing language → 422, invalid language → 422, missing source_code → 422 (all with nil store, validation happens first)
- [x] 4.2 Verify tests fail (RED)

## 5. Handler — GREEN

- [x] 5.1 Create `api/internal/handler/submissions.go` with `createSubmission` handler method on `ChallengesHandler`
- [x] 5.2 Wire `POST /{id}/submissions` into `ChallengesHandler.Routes`
- [x] 5.3 Map store errors: `ErrNotFound` → 404, others → 500; success → 202 with `{submission_id}`
- [x] 5.4 Verify handler tests pass (GREEN)

## 6. Server Bootstrap

- [x] 6.1 Add `github.com/redis/go-redis/v9` to `api/go.mod` (`go get github.com/redis/go-redis/v9`)
- [x] 6.2 Update `api/cmd/server/main.go` to read `REDIS_URL`, create `redis.Client`, pass to `store.New`
- [x] 6.3 Update `router.New` signature if needed (no change expected — store already wired)

## 7. Docker Compose & Smoke Test

- [x] 7.1 Add Redis service to `docker-compose.yml` if not present; add `REDIS_URL` env to API service
- [x] 7.2 Build and start the API in Docker; run migration 000003
- [x] 7.3 POST a valid submission and verify 202 response with `submission_id`
- [x] 7.4 Verify `submissions` row exists in PostgreSQL with `status = 'pending'`
- [x] 7.5 Verify Redis list `submissions` has one entry via `LRANGE submissions 0 -1`
