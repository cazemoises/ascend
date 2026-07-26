# Spec: Challenges CRUD

## Purpose

The challenges CRUD capability exposes REST endpoints for creating, retrieving, and deleting programming challenges and their associated test cases. It is the primary interface between the frontend and the challenge data stored in PostgreSQL.

## ADDED Requirements

### Requirement: List challenges endpoint
The API SHALL expose `GET /api/v1/challenges` that returns a JSON array of all challenges ordered by `created_at` descending. The endpoint SHALL support `limit` (default 50, max 100) and `offset` (default 0) query parameters for pagination.

#### Scenario: Challenges exist
- **WHEN** a client sends `GET /api/v1/challenges`
- **THEN** the server responds with HTTP 200 and a JSON array of challenge objects, each containing `id`, `slug`, `title`, `description`, `difficulty`, `time_limit_ms`, `memory_limit_mb`, `created_at`, `updated_at`

#### Scenario: No challenges exist
- **WHEN** a client sends `GET /api/v1/challenges` and the table is empty
- **THEN** the server responds with HTTP 200 and an empty JSON array `[]`

#### Scenario: Pagination via limit and offset
- **WHEN** a client sends `GET /api/v1/challenges?limit=10&offset=20`
- **THEN** the server responds with HTTP 200 and at most 10 challenges starting from position 20

### Requirement: Create challenge endpoint
The API SHALL expose `POST /api/v1/challenges` that accepts a JSON body and creates a new challenge. The request body SHALL include `slug` (string, required, unique), `title` (string, required), `description` (string, optional), `difficulty` (string, required, one of: `easy`, `medium`, `hard`), `time_limit_ms` (integer, optional, defaults to 2000), `memory_limit_mb` (integer, optional, defaults to 256). On success the server SHALL respond with HTTP 201 and the created challenge object.

#### Scenario: Valid challenge created
- **WHEN** a client sends `POST /api/v1/challenges` with a valid JSON body
- **THEN** the server responds with HTTP 201 and the full challenge object including the generated `id` and timestamps

#### Scenario: Duplicate slug rejected
- **WHEN** a client sends `POST /api/v1/challenges` with a `slug` that already exists
- **THEN** the server responds with HTTP 409 Conflict and a JSON error body

#### Scenario: Missing required field
- **WHEN** a client sends `POST /api/v1/challenges` with a missing `title` or `slug` or `difficulty`
- **THEN** the server responds with HTTP 422 Unprocessable Entity and a JSON error body

#### Scenario: Invalid difficulty value
- **WHEN** a client sends `POST /api/v1/challenges` with `"difficulty": "extreme"`
- **THEN** the server responds with HTTP 422 Unprocessable Entity and a JSON error body

#### Scenario: Malformed JSON body
- **WHEN** a client sends `POST /api/v1/challenges` with a non-JSON body
- **THEN** the server responds with HTTP 400 Bad Request and a JSON error body

### Requirement: Get challenge by ID endpoint
The API SHALL expose `GET /api/v1/challenges/{id}` that returns a single challenge by its UUID. If no challenge with that `id` exists the server SHALL respond with HTTP 404.

#### Scenario: Challenge found
- **WHEN** a client sends `GET /api/v1/challenges/{id}` with a valid existing UUID
- **THEN** the server responds with HTTP 200 and the challenge object

#### Scenario: Challenge not found
- **WHEN** a client sends `GET /api/v1/challenges/{id}` with a UUID that does not exist
- **THEN** the server responds with HTTP 404 and a JSON error body

#### Scenario: Invalid UUID format
- **WHEN** a client sends `GET /api/v1/challenges/not-a-uuid`
- **THEN** the server responds with HTTP 404 and a JSON error body

### Requirement: Delete challenge endpoint
The API SHALL expose `DELETE /api/v1/challenges/{id}` that hard-deletes a challenge and its associated test cases. If submissions reference the challenge the server SHALL respond with HTTP 409 Conflict. On success the server SHALL respond with HTTP 204 No Content.

#### Scenario: Challenge deleted successfully
- **WHEN** a client sends `DELETE /api/v1/challenges/{id}` for an existing challenge with no submissions
- **THEN** the challenge and all its test cases are deleted and the server responds with HTTP 204

#### Scenario: Challenge not found
- **WHEN** a client sends `DELETE /api/v1/challenges/{id}` with a UUID that does not exist
- **THEN** the server responds with HTTP 404 and a JSON error body

#### Scenario: Challenge has submissions
- **WHEN** a client sends `DELETE /api/v1/challenges/{id}` for a challenge referenced by submissions
- **THEN** the server responds with HTTP 409 Conflict and a JSON error body, and the challenge is not deleted

### Requirement: Add test case to challenge
The API SHALL expose `POST /api/v1/challenges/{id}/test-cases` that creates a new test case linked to the given challenge. The request body SHALL include `input` (string, optional), `expected_output` (string, required), `is_sample` (boolean, optional, defaults to false). The server SHALL assign the `ordinal` automatically as the next sequential value. On success the server SHALL respond with HTTP 201 and the created test case object.

#### Scenario: Test case created
- **WHEN** a client sends `POST /api/v1/challenges/{id}/test-cases` with a valid body for an existing challenge
- **THEN** the server responds with HTTP 201 and the test case object including `id`, `challenge_id`, `input`, `expected_output`, `is_sample`, and `ordinal`

#### Scenario: Ordinal assigned sequentially
- **WHEN** a second test case is created for the same challenge
- **THEN** the new test case has an `ordinal` one greater than the highest existing ordinal for that challenge

#### Scenario: Challenge not found
- **WHEN** a client sends `POST /api/v1/challenges/{id}/test-cases` with a non-existent challenge UUID
- **THEN** the server responds with HTTP 404 and a JSON error body

#### Scenario: Missing expected_output
- **WHEN** a client sends `POST /api/v1/challenges/{id}/test-cases` without `expected_output`
- **THEN** the server responds with HTTP 422 Unprocessable Entity and a JSON error body

### Requirement: List test cases for a challenge
The API SHALL expose `GET /api/v1/challenges/{id}/test-cases` that returns a JSON array of all test cases for the given challenge, ordered by `ordinal` ascending.

#### Scenario: Test cases returned in ordinal order
- **WHEN** a client sends `GET /api/v1/challenges/{id}/test-cases` for an existing challenge with test cases
- **THEN** the server responds with HTTP 200 and a JSON array of test case objects ordered by `ordinal` ascending

#### Scenario: No test cases
- **WHEN** a client sends `GET /api/v1/challenges/{id}/test-cases` for a challenge with no test cases
- **THEN** the server responds with HTTP 200 and an empty JSON array `[]`

#### Scenario: Challenge not found
- **WHEN** a client sends `GET /api/v1/challenges/{id}/test-cases` with a non-existent challenge UUID
- **THEN** the server responds with HTTP 404 and a JSON error body
