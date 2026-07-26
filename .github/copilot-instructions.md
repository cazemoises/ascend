mkdir -p /mnt/c/Users/dev/Documents/projects/ascend/.github
cat > /mnt/c/Users/dev/Documents/projects/ascend/.github/copilot-instructions.md << 'EOF'
# Ascend — Copilot Instructions

## Project
HackerRank-style online code judge. Go API + Judge worker + React frontend.

## Stack
- Backend: Go (stdlib-first, chi router)
- Frontend: React + TypeScript (Vite)
- DB: PostgreSQL, Queue: Redis, Sandbox: Docker

## Architecture
[React] → [Go API] → [Redis queue] → [Judge worker] → [Docker sandbox]
                ↓                            ↓
          [PostgreSQL]              [PostgreSQL results]

## Conventions
- Go: errors with fmt.Errorf("context: %w", err); no panics; table-driven tests
- Go: one package per concern; no global state
- TS: strict mode; no `any`; functional components + hooks
- SQL: parameterized queries only; migrations append-only
- API: RESTful JSON under /api/v1/
- Commits: conventional commits

## Workflow
- Plan before coding — show the plan first, wait for approval
- Write tests before implementation (RED-GREEN)
- Smallest change that satisfies the task
- Do NOT run tests yourself — give me the command and I'll run it
- Do NOT loop retrying commands — if something fails, explain and wait

## Do NOT read
- node_modules/, web/dist/, vendor/, *.log, tmp/, openspec/archive/

## Current state (June 2026)
- ✅ Go API: /healthz, CRUD /challenges, POST /submissions (Redis queue)
- ✅ Judge worker: BLPOP stub (dequeues but doesn't execute)
- ✅ PostgreSQL schema: users, challenges, submissions, test_cases
- ✅ Migration runner: cmd/migrate (up/down/version)
- 🔲 GET /submissions/{id}
- 🔲 Judge real (Docker execution)
- 🔲 Frontend (placeholder only)

## Test commands (YOU give me, I run)
- Go tests: cd api && go test ./... (run in WSL Ubuntu)
- Go vet: go vet ./...
- Docker rebuild: docker compose build api && docker compose up -d api
- Smoke test: curl from WSL bash, never PowerShell
EOF