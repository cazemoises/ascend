## ADDED Requirements

### Requirement: Worker connects to Redis on startup
The judge worker SHALL read the Redis address from `REDIS_ADDR` (default `localhost:6379`) and establish a connection at startup. If the connection fails, the worker SHALL log the error and exit non-zero.

#### Scenario: Successful Redis connection
- **WHEN** Redis is reachable at the configured address
- **THEN** the worker logs "connected to redis" and begins polling

#### Scenario: Redis unreachable at startup
- **WHEN** Redis is not reachable at the configured address
- **THEN** the worker logs the connection error and exits with a non-zero status code

### Requirement: Worker polls the submission queue
The worker SHALL BLPOP on the `submissions` Redis list key with a timeout, loop on empty results, and process each submission message it dequeues.

#### Scenario: Message dequeued
- **WHEN** a message is present on the `submissions` key
- **THEN** the worker dequeues it and logs the submission ID at INFO level

#### Scenario: Queue empty — worker continues polling
- **WHEN** the `submissions` key is empty for the BLPOP timeout duration
- **THEN** the worker loops and polls again without exiting

### Requirement: Graceful shutdown on SIGTERM
The worker SHALL catch SIGTERM (and SIGINT), finish processing the current submission if one is in progress, and exit cleanly.

#### Scenario: Shutdown while idle
- **WHEN** SIGTERM is received while the worker is blocked on BLPOP
- **THEN** the worker exits within 1 second

#### Scenario: Shutdown while processing
- **WHEN** SIGTERM is received while processing a submission
- **THEN** the worker completes the current submission before exiting

### Requirement: Structured JSON logging
The worker SHALL emit all log output as JSON lines to stdout, including at minimum `level`, `msg`, and `ts` fields.

#### Scenario: Log output is valid JSON
- **WHEN** the worker emits any log line
- **THEN** the line parses as valid JSON with `level`, `msg`, and `ts` fields present
