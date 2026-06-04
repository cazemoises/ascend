## ADDED Requirements

### Requirement: List recent submissions for a challenge
The API SHALL expose `GET /api/v1/challenges/{id}/submissions` that returns the last 10 submissions for the given challenge ordered by `created_at` descending. Each object SHALL include `id`, `status`, `language`, and `created_at`. Source code and private test case details SHALL NOT be included in this response. The endpoint does not require authentication at this stage.

#### Scenario: Submissions exist
- **WHEN** a client sends `GET /api/v1/challenges/{id}/submissions` for a challenge with submissions
- **THEN** the server responds with HTTP 200 and a JSON array of at most 10 submission summaries, each containing `id`, `status`, `language`, `created_at`, ordered newest first

#### Scenario: No submissions exist
- **WHEN** a client sends `GET /api/v1/challenges/{id}/submissions` for a challenge with no submissions
- **THEN** the server responds with HTTP 200 and an empty JSON array `[]`

#### Scenario: Challenge not found
- **WHEN** a client sends `GET /api/v1/challenges/{id}/submissions` with a non-existent challenge UUID
- **THEN** the server responds with HTTP 404 and a JSON error body

#### Scenario: More than 10 submissions exist
- **WHEN** a challenge has more than 10 submissions
- **THEN** the endpoint returns exactly the 10 most recent submissions by `created_at` descending

### Requirement: Challenge page displays submission history
The challenge page (`/challenges/:id`) SHALL render a submission history panel below the code editor. The panel SHALL show at most 10 recent submissions fetched from `GET /api/v1/challenges/:id/submissions`. Each row SHALL display the status (with colour-coding matching the verdict styles), language, and relative time.

#### Scenario: History loads after the challenge
- **WHEN** a user navigates to `/challenges/:id` and the challenge has submissions
- **THEN** a submissions table is displayed below the editor with columns Status, Language, and Date

#### Scenario: Empty history
- **WHEN** the challenge has no submissions
- **THEN** no table is rendered and a "Sem submissões ainda." message is shown in the history section

#### Scenario: History refreshes after submit
- **WHEN** the user submits a solution and is redirected to the result page, then returns to the challenge page
- **THEN** the newly submitted attempt appears in the history list on reload
