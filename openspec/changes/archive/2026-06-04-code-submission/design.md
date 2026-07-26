## Context

The Ascend API already has challenges CRUD and a PostgreSQL store layer. The judge worker reads from a Redis list and executes code in Docker sandboxes. The missing piece is the submission entry point: an endpoint that validates input, persists a record, and enqueues the job so the worker can pick it up.

## Goals / Non-Goals

**Goals:**
- Accept a code submission via REST, persist it with `status: pending`, enqueue to Redis
- Return `202 Accepted` with `submission_id` immediately (fire-and-forget)
- Support Go, Python, and JavaScript as valid languages

**Non-Goals:**
- Polling/webhook for result retrieval (future change)
- Rate limiting or quota enforcement
- Code size limits beyond basic validation
- Authentication/authorization

## Decisions

### Redis client: `go-redis/redis/v9`

The judge worker will use the same library. Using `go-redis` over raw `net` commands gives typed errors, connection pooling, and context propagation. The `REDIS_URL` env var is parsed via `redis.ParseURL` for consistency with how `DATABASE_URL` works.

**Alternative considered:** `github.com/gomodule/redigo` — rejected; go-redis has better context and generic type support.

### Enqueue via `LPUSH submissions <json>`

The worker pops with `BRPOP submissions` (blocking right-pop). Using a Redis list (LPUSH/BRPOP) is the simplest reliable queue with no extra infra. The payload is `{"submission_id":"<uuid>","challenge_id":"<uuid>"}`.

**Alternative considered:** Redis Streams — more powerful but unnecessary complexity for the current scale.

### Submissions table schema

```sql
CREATE TABLE submissions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    challenge_id  UUID NOT NULL REFERENCES challenges(id),
    language      TEXT NOT NULL,
    source_code   TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`status` is a plain `TEXT` column (not enum) for forward compatibility — the judge adds `running`, `accepted`, `wrong_answer`, `time_limit_exceeded`, `runtime_error`, `compile_error` without a migration.

### Store layer extended — `SubmissionStore` in same package

Adding `CreateSubmission` to the existing `store` package keeps the dependency graph flat. The `Store` struct gains a Redis client field so `CreateSubmission` can enqueue atomically in a single method call (insert → enqueue in the same function, not a transaction — acceptable because a crash between the two leaves a pending row that can be retried later).

### Handler wires into existing router

`POST /{challengeID}/submissions` is added under the existing `/api/v1/challenges/{id}` route group handled by `ChallengesHandler`. This avoids a new handler type for a single endpoint.

## Risks / Trade-offs

- **Insert succeeds, enqueue fails**: Row stays `pending` forever — acceptable for now; a background sweep can re-enqueue orphaned rows in a future change.
- **Invalid UUID for `{id}`**: PostgreSQL rejects it at the query level; the store maps the error to `ErrNotFound` (FK check handles it).
- **No idempotency key**: Duplicate submissions are allowed; each POST creates a new row. Acceptable for a judge system.

## Migration Plan

1. Run `go run ./cmd/migrate up` — applies `000003_submissions.up.sql`
2. Start `REDIS_URL` env var alongside `DATABASE_URL` in docker-compose
3. Rollback: `go run ./cmd/migrate down -steps 1`
