## Why

The judge pipeline needs an entry point: users must be able to submit code solutions for evaluation. Without a submission endpoint, the Redis queue and judge worker have no source of work.

## What Changes

- New `POST /api/v1/challenges/{id}/submissions` endpoint accepting `language` and `source_code`
- New `submissions` table to persist submission records with `status: pending`
- New migration `000003_submissions` creating the `submissions` table
- Submission job enqueued to Redis list `submissions` as JSON `{submission_id, challenge_id}`
- Endpoint returns `202 Accepted` with `{submission_id}`
- Redis client added to the API server (`go-redis/redis/v9`)

## Capabilities

### New Capabilities

- `code-submission`: REST endpoint to accept a code submission, persist it, and enqueue it for judging

### Modified Capabilities

- `database-schema`: `submissions` table added (new requirement)

## Impact

- **API**: new endpoint under `/api/v1/challenges/{id}/submissions`
- **Database**: new `submissions` table (`000003` migration)
- **Redis**: API server gains a Redis client; enqueues to `submissions` list
- **Dependencies**: `github.com/redis/go-redis/v9` added to `api/go.mod`
- **Server bootstrap**: `REDIS_URL` env var read alongside `DATABASE_URL`
