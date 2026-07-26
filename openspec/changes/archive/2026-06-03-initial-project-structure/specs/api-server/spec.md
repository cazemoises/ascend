## ADDED Requirements

### Requirement: Server starts and binds to a configurable port
The API server SHALL read its listening address from the `API_ADDR` environment variable (default `0.0.0.0:8080`) and bind an HTTP listener on that address at startup.

#### Scenario: Default port binding
- **WHEN** the server starts without `API_ADDR` set
- **THEN** it listens on `0.0.0.0:8080`

#### Scenario: Custom port via env var
- **WHEN** `API_ADDR=0.0.0.0:9090` is set
- **THEN** the server listens on port 9090

### Requirement: Health check endpoint
The server SHALL expose `GET /healthz` that returns HTTP 200 and a JSON body `{"status":"ok"}` when the server is running.

#### Scenario: Healthy response
- **WHEN** a client sends `GET /healthz`
- **THEN** the server responds with HTTP 200 and `{"status":"ok"}`

### Requirement: Graceful shutdown on SIGTERM
The server SHALL catch SIGTERM (and SIGINT) and complete in-flight requests before exiting, with a configurable drain timeout (default 10 seconds).

#### Scenario: In-flight request completes before shutdown
- **WHEN** SIGTERM is received while a request is in flight
- **THEN** the server waits for the request to finish before exiting

#### Scenario: Drain timeout exceeded
- **WHEN** SIGTERM is received and a request exceeds the drain timeout
- **THEN** the server forcefully closes the connection and exits

### Requirement: Structured JSON request logging
The server SHALL log each request as a JSON line including method, path, status code, latency, and request ID.

#### Scenario: Request log line emitted
- **WHEN** any HTTP request is handled
- **THEN** a JSON log line with `method`, `path`, `status`, `latency_ms`, and `request_id` fields is written to stdout

### Requirement: API routes versioned under /api/v1/
All application routes SHALL be mounted under the `/api/v1/` prefix. The health endpoint is exempt from versioning.

#### Scenario: Unversioned path returns 404
- **WHEN** a client requests `/api/challenges` (no version prefix)
- **THEN** the server responds with HTTP 404

#### Scenario: Versioned path is routable
- **WHEN** a client requests `/api/v1/challenges`
- **THEN** the router dispatches to the challenges handler (once implemented)
