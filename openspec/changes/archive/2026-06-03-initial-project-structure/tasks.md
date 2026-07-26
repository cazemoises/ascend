## 1. Repository Scaffolding

- [x] 1.1 Create `go.work` file at repo root declaring `api/` and `judge/` modules (and top-level `pkg/` module)
- [x] 1.2 Create `.env.example` documenting all environment variables with safe defaults
- [x] 1.3 Create `.env` (git-ignored) copied from `.env.example` for local use
- [x] 1.4 Create `Makefile` with `dev`, `down`, `test`, `migrate`, and `build` targets
- [x] 1.5 Update `.gitignore` to exclude `.env`, `bin/`, `web/dist/`, `vendor/`, `*.log`, `tmp/`

## 2. Database Migrations

- [x] 2.1 Create `migrations/` directory with `000001_initial_schema.up.sql` defining `users`, `challenges`, `submissions`, `test_cases` tables (UUIDs, enums, FK constraints, timestamps)
- [x] 2.2 Create `migrations/000001_initial_schema.down.sql` that drops all four tables in reverse dependency order
- [x] 2.3 Create `cmd/migrate/main.go` using `golang-migrate` to apply or roll back migrations from `DATABASE_URL` env var
- [x] 2.4 Run `go run ./cmd/migrate up` against a local postgres and confirm the schema is created

## 3. Go API Server (`api/`)

- [x] 3.1 Initialize Go module `github.com/caze/ascend/api` with `go mod init` inside `api/`
- [x] 3.2 Add `chi` and `log/slog` (stdlib) dependencies; run `go mod tidy`
- [x] 3.3 Create `api/cmd/server/main.go` — reads `API_ADDR` env var, constructs router, starts HTTP server, handles SIGTERM with 10s drain
- [x] 3.4 Create `api/internal/router/router.go` — mounts middleware (request ID, structured JSON logger) and registers `/healthz` and `/api/v1/` prefix
- [x] 3.5 Create `api/internal/handler/health.go` — `GET /healthz` returns `{"status":"ok"}` with HTTP 200
- [x] 3.6 Write table-driven tests in `api/internal/handler/health_test.go` verifying 200 status and response body
- [x] 3.7 Run `go test ./...` in `api/` and confirm all tests pass

## 4. Judge Worker (`judge/`)

- [x] 4.1 Initialize Go module `github.com/caze/ascend/judge` with `go mod init` inside `judge/`
- [x] 4.2 Add `github.com/redis/go-redis/v9` dependency; run `go mod tidy`
- [x] 4.3 Create `judge/cmd/worker/main.go` — reads `REDIS_ADDR` env var, connects to Redis (exit non-zero on failure), starts poll loop, handles SIGTERM
- [x] 4.4 Create `judge/internal/worker/worker.go` — BLPOP loop on `submissions` key, logs submission ID at INFO, placeholder for actual processing
- [x] 4.5 Write unit tests in `judge/internal/worker/worker_test.go` using a mock Redis client
- [x] 4.6 Run `go test ./...` in `judge/` and confirm all tests pass

## 5. React Frontend (`web/`)

- [x] 5.1 Scaffold project with `npm create vite@latest web -- --template react-ts` at repo root
- [x] 5.2 Configure `vite.config.ts` to proxy `/api/` requests to `http://localhost:8080`
- [x] 5.3 Update `tsconfig.json` to ensure `strict: true` is set
- [x] 5.4 Replace default `App.tsx` with a minimal placeholder home page that renders the text "Ascend"
- [x] 5.5 Run `npm run build` in `web/` and confirm `web/dist/index.html` is produced with no errors

## 6. Docker Setup

- [x] 6.1 Create `api/Dockerfile` — multi-stage build: `golang:1.23-alpine` builder → `alpine` runtime; copies binary, sets `ENTRYPOINT`
- [x] 6.2 Create `judge/Dockerfile` — same pattern as api Dockerfile
- [x] 6.3 Create `web/Dockerfile` — multi-stage: `node:22-alpine` builder runs `npm run build` → `nginx:alpine` serves `dist/`
- [x] 6.4 Create `docker/sandbox/Dockerfile` — minimal `alpine` image with common runtimes placeholder (python3, gcc); this image is built but not run by compose
- [x] 6.5 Create `docker-compose.yml` with services: `postgres` (healthcheck on `pg_isready`), `redis` (healthcheck on `redis-cli ping`), `api` (depends_on postgres healthy), `judge` (depends_on redis healthy), `web` (no deps)
- [x] 6.6 Configure all service environment variables in compose to read from `.env` file
- [x] 6.7 Run `docker compose up --build` and confirm all five services reach running/healthy state

## 7. Integration Verification

- [x] 7.1 Run `make migrate` and confirm schema tables exist in the running postgres container
- [x] 7.2 Curl `http://localhost:8080/healthz` and confirm `{"status":"ok"}` response
- [x] 7.3 Open `http://localhost:5173` in a browser and confirm the Ascend placeholder page renders without console errors
- [x] 7.4 Run `make test` and confirm both Go and frontend test suites pass
- [x] 7.5 Run `make down` and confirm all containers stop cleanly
