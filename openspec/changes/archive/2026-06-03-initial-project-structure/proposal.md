## Why

Ascend has no code yet — only a spec, a CLAUDE.md, and an empty git repo. The project needs a working monorepo skeleton so development can begin: runnable services, a wired-up local environment, and enough scaffolding for the first real feature to land cleanly.

## What Changes

- Add `api/` — Go module with chi router, health endpoint, and server entry point
- Add `judge/` — Go module for the judge worker (queue consumer skeleton)
- Add `web/` — Vite + React + TypeScript SPA skeleton
- Add `migrations/` — initial PostgreSQL schema (users, challenges, submissions)
- Add `docker-compose.yml` — local dev environment (postgres, redis, api, judge, web)
- Add `docker/` — Dockerfile for sandbox execution environment
- Add `Makefile` — top-level shortcuts (`make dev`, `make test`, `make migrate`)
- Shared `go.work` workspace so `api` and `judge` can share internal packages

## Capabilities

### New Capabilities
- `api-server`: Go HTTP API — routing, middleware, health check, server lifecycle
- `judge-worker`: Redis queue consumer — picks up submissions, runs sandbox, writes results
- `web-frontend`: React SPA skeleton — Vite config, router, placeholder pages
- `local-dev-environment`: Docker Compose stack — all services wired for local development
- `database-schema`: PostgreSQL schema — users, challenges, submissions, test cases tables

### Modified Capabilities
<!-- none — greenfield -->

## Impact

- Introduces the root directory structure every subsequent feature will build on
- `go.work` workspace affects all future Go module additions
- `docker-compose.yml` is the canonical way to run services locally
- DB migrations are append-only from this point forward
- No external dependencies beyond the stack defined in CLAUDE.md (Go, Node, Postgres, Redis, Docker)
