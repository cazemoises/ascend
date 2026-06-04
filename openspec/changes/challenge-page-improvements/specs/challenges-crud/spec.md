## MODIFIED Requirements

### Requirement: Create challenge endpoint
The API SHALL expose `POST /api/v1/challenges` that accepts a JSON body and creates a new challenge. The request body SHALL include `slug` (string, required, unique), `title` (string, required), `description` (string, optional), `difficulty` (string, required, one of: `easy`, `medium`, `hard`), `time_limit_ms` (integer, optional, defaults to 2000), `memory_limit_mb` (integer, optional, defaults to 256), `notes` (string, optional, nullable — instructional text displayed on the challenge page). On success the server SHALL respond with HTTP 201 and the created challenge object including the `notes` field.

#### Scenario: Valid challenge created
- **WHEN** a client sends `POST /api/v1/challenges` with a valid JSON body
- **THEN** the server responds with HTTP 201 and the full challenge object including the generated `id`, timestamps, and `notes`

#### Scenario: Challenge created with notes
- **WHEN** a client sends `POST /api/v1/challenges` with `"notes": "Leia dois inteiros do stdin e imprima a soma."`
- **THEN** the server responds with HTTP 201 and the challenge object includes `"notes": "Leia dois inteiros do stdin e imprima a soma."`

#### Scenario: Challenge created without notes
- **WHEN** a client sends `POST /api/v1/challenges` without a `notes` field
- **THEN** the server responds with HTTP 201 and the challenge object includes `"notes": null`

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

### Requirement: List challenges endpoint
The API SHALL expose `GET /api/v1/challenges` that returns a JSON array of all challenges ordered by `created_at` descending. The endpoint SHALL support `limit` (default 50, max 100) and `offset` (default 0) query parameters for pagination. Each challenge object SHALL include the `notes` field (string or null).

#### Scenario: Challenges exist
- **WHEN** a client sends `GET /api/v1/challenges`
- **THEN** the server responds with HTTP 200 and a JSON array of challenge objects, each containing `id`, `slug`, `title`, `description`, `difficulty`, `time_limit_ms`, `memory_limit_mb`, `notes`, `created_at`, `updated_at`

#### Scenario: No challenges exist
- **WHEN** a client sends `GET /api/v1/challenges` and the table is empty
- **THEN** the server responds with HTTP 200 and an empty JSON array `[]`

#### Scenario: Pagination via limit and offset
- **WHEN** a client sends `GET /api/v1/challenges?limit=10&offset=20`
- **THEN** the server responds with HTTP 200 and at most 10 challenges starting from position 20

### Requirement: Get challenge by ID endpoint
The API SHALL expose `GET /api/v1/challenges/{id}` that returns a single challenge by its UUID with `sample_test_cases` embedded. The response SHALL include the `notes` field (string or null). If no challenge with that `id` exists the server SHALL respond with HTTP 404.

#### Scenario: Challenge found
- **WHEN** a client sends `GET /api/v1/challenges/{id}` with a valid existing UUID
- **THEN** the server responds with HTTP 200 and the challenge object including `notes` and `sample_test_cases`

#### Scenario: Challenge not found
- **WHEN** a client sends `GET /api/v1/challenges/{id}` with a UUID that does not exist
- **THEN** the server responds with HTTP 404 and a JSON error body

#### Scenario: Invalid UUID format
- **WHEN** a client sends `GET /api/v1/challenges/not-a-uuid`
- **THEN** the server responds with HTTP 404 and a JSON error body
