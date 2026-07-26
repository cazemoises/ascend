## ADDED Requirements

### Requirement: docker compose up starts all services
Running `docker compose up` at the repo root SHALL start postgres, redis, api, judge, and web services. All services SHALL reach a healthy or running state without manual intervention.

#### Scenario: All services start successfully
- **WHEN** `docker compose up --build` is run from the repo root
- **THEN** all five services (postgres, redis, api, judge, web) are running with no immediate exit

#### Scenario: API waits for postgres health
- **WHEN** postgres has not yet passed its healthcheck
- **THEN** the api service does not start (depends_on with condition: service_healthy)

#### Scenario: Judge waits for redis health
- **WHEN** redis has not yet passed its healthcheck
- **THEN** the judge service does not start (depends_on with condition: service_healthy)

### Requirement: Services connect via Docker Compose network
All services SHALL communicate over a shared Docker Compose network using service names as hostnames (e.g., `postgres:5432`, `redis:6379`).

#### Scenario: API resolves postgres by hostname
- **WHEN** the api service connects to the database
- **THEN** it uses `DATABASE_URL=postgres://ascend:ascend@postgres:5432/ascend?sslmode=disable`

#### Scenario: Judge resolves redis by hostname
- **WHEN** the judge worker connects to redis
- **THEN** it uses `REDIS_ADDR=redis:6379`

### Requirement: Environment variables are configurable via .env file
The compose file SHALL read from a `.env` file at the repo root. A `.env.example` SHALL document all required variables. `.env` SHALL be git-ignored.

#### Scenario: .env.example documents all variables
- **WHEN** a developer clones the repo
- **THEN** `.env.example` lists every env var with a safe default value

### Requirement: Makefile shortcuts wrap common commands
A `Makefile` at the repo root SHALL provide: `make dev` (docker compose up), `make down` (docker compose down), `make test` (go test + npm test), `make migrate` (run DB migrations), `make build` (build all Docker images).

#### Scenario: make dev starts the stack
- **WHEN** a developer runs `make dev`
- **THEN** `docker compose up -d` is executed
