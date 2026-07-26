# CLAUDE.md — Ascend

## Project
Ascend is a HackerRank-style online code judge for real users.
Users submit code solutions to programming challenges; the system executes
them in a Docker sandbox, runs test cases, and returns a verdict.

## Stack
- Backend API: Go (stdlib-first; chi router)
- Frontend: React + TypeScript (Vite)
- Database: PostgreSQL
- Queue / cache: Redis (submission queue)
- Sandbox: Docker (isolated execution per submission via docker CLI)

## Architecture
```
[React SPA] → [Go API] → [Redis queue] → [Judge worker] → [Docker sandbox]
                ↓                               ↓
           [PostgreSQL]               [PostgreSQL results]
```

## Current state (July 2026)
### API endpoints (all working)
- GET  /healthz
- POST /api/v1/auth/register           — bcrypt + JWT, returns {token, user}
- POST /api/v1/auth/login              — returns {token, user}
- GET  /api/v1/challenges              — list with pagination
- POST /api/v1/challenges              — create
- GET  /api/v1/challenges/:id          — detail + sample_test_cases (is_sample=true)
- DELETE /api/v1/challenges/:id        — delete (409 if has submissions)
- POST /api/v1/challenges/:id/test-cases
- GET  /api/v1/challenges/:id/test-cases
- POST /api/v1/challenges/:id/submissions — JWT + rate limited, 202, stamps user_id
- GET  /api/v1/submissions             — JWT; user history, keyset cursor pagination
- GET  /api/v1/submissions/:id         — polling endpoint

### Auth & rate limiting (working)
- Stdlib HS256 JWT (api/internal/auth): Sign/Verify/Middleware; JWT_SECRET env (≥32 bytes) required at boot
- bcrypt password hashing; login timing equalized against unknown emails
- Redis sliding-window limiter (api/internal/middleware): ZSET per user on POST submissions;
  SUBMIT_RATE_LIMIT (10) / SUBMIT_RATE_WINDOW_S (60); fails open if Redis down; tests use miniredis

### Judge worker (working)
- BLPOP from Redis submissions queue
- Fetches submission + challenge + test_cases from PostgreSQL
- Executes code via: docker run --rm -i --network none --memory Xm --cpus 0.5 {image}
- Code injected via tar stream on stdin (avoids Docker-in-Docker bind mount issues)
- Languages: python:3.11-alpine, node:20-alpine, golang:1.26-alpine
- Verdicts: accepted, wrong_answer, runtime_error, time_limit_exceeded
- Fail-fast: stops on first failing test case

### Frontend (working)
- /                              — challenge list
- /login, /register              — auth pages; AuthContext + localStorage token, Bearer injected in api.ts
- /challenges/:id                — challenge detail + Monaco editor (@monaco-editor/react) + submit
- /challenges/:id/submissions/:subId — result with 2s polling until status != pending
- /submissions                   — user history, cursor-paginated "load more"
- 401 responses clear the session (ascend:unauthorized event)
- CORS configured for localhost:5173 and :5174 (dev only; prod is same-origin via nginx)

### Production deploy (working)
- api/Dockerfile: multi-stage, static stripped binary, non-root user, healthcheck
- web/Dockerfile: npm ci build → nginx:alpine; VITE_API_URL="" build arg = same-origin
- docker/nginx.conf: SPA fallback, gzip, immutable /assets/ caching, /api + /healthz proxied to api:8080
- docker-compose: healthchecks + service_healthy ordering for api and web

### Database migrations
- 000001: users, challenges, submissions, test_cases (with is_sample boolean)
- 000002: ADD time_limit_ms, memory_limit_mb to challenges
- 000003: submissions table (TEXT status, no user_id yet)
- 000004: ADD notes to challenges
- 000006: unique index on LOWER(users.email)
- 000007: submissions.user_id (nullable FK) + (user_id, created_at DESC, id DESC) index

### What's NOT implemented yet
- Ranking / leaderboard

## Read these
- `api/`, `judge/`, `web/src/`
- `migrations/` only when changing schema
- `docker/` when touching sandbox config

## Do NOT read (context pollution)
- `node_modules/`, `web/dist/`, `web/build/`
- `vendor/`, `bin/`, `*.log`, `tmp/`, `coverage/`
- Generated mocks and `*_gen.go`

## Conventions
- Go: errors wrapped with `fmt.Errorf("context: %w", err)`; no panics in request path
- Go: one package per concern; no global state; table-driven tests
- TS: strict mode; no `any`; functional components + hooks only
- SQL: parameterized queries only; migrations append-only
- API: RESTful JSON under /api/v1/
- Commits: conventional commits (feat:, fix:, refactor:, test:, chore:)

## Workflow (mandatory)
1. Identify the feature or bug clearly
2. Plan the code implementation directly in chat with a short summary
3. Implement changes in small, logical, incremental steps
4. Run tests (`go test ./...` or `npm test`) to ensure stability
5. Use subagents for codebase exploration; keep main context for implementation
6. Smallest change that satisfies the task; do not touch unrelated code

## Smoke tests
Always use curl from WSL/bash. Never PowerShell for HTTP tests.
Example: curl -s -X POST http://localhost:8080/api/v1/challenges \
  -H "Content-Type: application/json" -d '{}' | jq

## Compaction
When compacting, always preserve:
- Current task goal
- List of modified files
- Test/run commands in use

## Commands
All Go commands run from WSL Ubuntu (`wsl.exe -d Ubuntu`) — the Windows Go
toolchain is blocked by an Application Control policy.
- API dev:      go run ./api/cmd/server (needs JWT_SECRET ≥32 bytes in env)
- Judge dev:    go run ./judge/cmd/worker
- Test Go:      go test ./...
- Test web:     cd web && npm test
- Lint Go:      golangci-lint run
- Lint web:     cd web && npm run lint
- DB migrate:   docker run --rm -v $(pwd):/src -w /src --network ascend_default \
                  -e DATABASE_URL=postgres://ascend:ascend@postgres:5432/ascend?sslmode=disable \
                  golang:1.26-alpine go run ./cmd/migrate up
- Docker up:    docker compose up -d
- Rebuild API:  docker compose build api && docker compose up -d api
- Rebuild judge: docker compose build judge && docker compose up -d judge