## Context

Ascend is a greenfield project. The repository has a CLAUDE.md and an openspec directory but no source code. This design establishes the monorepo skeleton: directory layout, module boundaries, inter-service wiring, and local dev ergonomics. Every subsequent feature will build on top of what this change creates.

## Goals / Non-Goals

**Goals:**
- Runnable `docker compose up` that starts all services (api, judge, web, postgres, redis)
- Go workspace unifying `api/` and `judge/` modules so shared packages work without a publish step
- React + TypeScript SPA booting with Vite dev server, able to proxy API calls
- PostgreSQL with initial schema (users, challenges, submissions, test_cases) and a migration runner
- `Makefile` top-level shortcuts to reduce friction (`make dev`, `make test`, `make migrate`)
- All services emit structured JSON logs and respond to SIGTERM gracefully

**Non-Goals:**
- Authentication / session management (next feature)
- Challenge CRUD API endpoints (next feature)
- Actual judge logic / Docker sandbox execution (later feature)
- Production deployment, CI/CD pipelines, TLS
- Frontend routing beyond a single placeholder page

## Decisions

### 1. Go workspace (`go.work`) over a single root module
`api/` and `judge/` are distinct binaries with different dependency trees. A `go.work` workspace lets them share an internal `pkg/` without publishing to a module proxy. Alternative: single module with `internal/` sub-packages — rejected because it couples api and judge release cycles and complicates future extraction.

### 2. `chi` router for the API
`chi` is stdlib-compatible (`net/http`), has a clean middleware story, and adds zero magic. Alternative: `gin` (more batteries, heavier, strays from stdlib patterns) — rejected per CLAUDE.md "stdlib-first" convention.

### 3. `golang-migrate` for database migrations
SQL-first, file-based, append-only — matches the project convention exactly. Alternative: GORM auto-migrate — rejected because it hides schema state and conflicts with the append-only rule.

### 4. Vite for the React frontend
First-class TypeScript support, fast HMR, simple proxy config for API calls in dev. Alternative: CRA — deprecated upstream.

### 5. Docker Compose `depends_on` with healthchecks
Services (api, judge) wait for postgres and redis to be healthy before starting. Alternative: retry loops in application code — rejected; healthchecks keep startup logic out of business code.

### 6. Sandbox Dockerfile is a separate image (`docker/sandbox/Dockerfile`)
The judge worker will `docker run` sandbox containers at runtime. Keeping the sandbox image separate from the judge service image limits blast radius if the sandbox is compromised. The sandbox image is built as part of `docker compose build` but never started directly by compose.

## Risks / Trade-offs

- **`go.work` is local-only** → CI must run `go work sync` or use `-workfile` flag; add to Makefile and document in README.
- **Vite proxy in dev ≠ production routing** → production will need a reverse proxy (nginx); not a concern now but must not bake assumptions into the SPA router.
- **Empty judge worker** → the worker binary will compile and run but do nothing useful; this is intentional scaffolding, not a regression risk.
- **Migrations run at startup** → acceptable for dev; production will need a separate migration job before rollout (document this when CI is added).

## Migration Plan

This is a greenfield change — no existing data or running services to migrate.

1. Scaffold files as specified in tasks.md
2. Run `docker compose up --build` and verify all containers reach healthy state
3. Verify `make migrate` applies the initial schema
4. Verify `go test ./...` passes in both `api/` and `judge/`
5. Verify `cd web && npm test` passes

Rollback: `git revert` — no production state to unwind.

## Open Questions

- Should `pkg/` shared Go code live under `api/pkg/` (imported by judge) or as a top-level `pkg/` module in the workspace? Proposal: top-level `pkg/` module for clarity. Revisit if it adds friction.
- Redis auth in local dev: password or no password? Proposal: no password in compose, configure via env var so production can set one.
