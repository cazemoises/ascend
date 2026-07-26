## ADDED Requirements

### Requirement: Initial schema created by versioned migrations
The database schema SHALL be managed exclusively through numbered migration files in `migrations/`. Migrations SHALL be applied in order and are append-only. The migration runner SHALL be invoked via `go run ./cmd/migrate up`.

#### Scenario: Migration runner applies pending migrations
- **WHEN** `go run ./cmd/migrate up` is run against a fresh database
- **THEN** all migration files are applied in numeric order and the schema matches the expected state

#### Scenario: Migration runner is idempotent on repeated runs
- **WHEN** `go run ./cmd/migrate up` is run a second time
- **THEN** no migrations are re-applied and the runner exits with code 0

### Requirement: users table stores account records
The schema SHALL include a `users` table with at minimum: `id` (UUID primary key), `email` (unique, not null), `password_hash` (not null), `created_at`, `updated_at`.

#### Scenario: User row can be inserted
- **WHEN** a row is inserted into `users` with a unique email
- **THEN** the row persists with a generated UUID and timestamps

#### Scenario: Duplicate email is rejected
- **WHEN** a row is inserted into `users` with an email that already exists
- **THEN** the database returns a unique constraint violation error

### Requirement: challenges table stores problem definitions
The schema SHALL include a `challenges` table with: `id` (UUID primary key), `slug` (unique, not null), `title` (not null), `description` (text), `difficulty` (enum: easy/medium/hard), `created_at`, `updated_at`.

#### Scenario: Challenge row can be inserted
- **WHEN** a row is inserted into `challenges` with a unique slug
- **THEN** the row persists with the correct difficulty value

### Requirement: submissions table stores code submissions
The schema SHALL include a `submissions` table with: `id` (UUID primary key), `user_id` (FK → users), `challenge_id` (FK → challenges), `language` (not null), `code` (text, not null), `status` (enum: pending/running/accepted/wrong_answer/error), `created_at`, `updated_at`.

#### Scenario: Submission row linked to user and challenge
- **WHEN** a submission is inserted referencing valid user and challenge IDs
- **THEN** the row persists with status `pending`

#### Scenario: Submission with invalid user_id is rejected
- **WHEN** a submission is inserted with a non-existent user_id
- **THEN** the database returns a foreign key constraint violation

### Requirement: test_cases table stores per-challenge test data
The schema SHALL include a `test_cases` table with: `id` (UUID primary key), `challenge_id` (FK → challenges), `input` (text), `expected_output` (text), `is_sample` (boolean), `ordinal` (integer for ordering).

#### Scenario: Test case linked to challenge
- **WHEN** a test case is inserted referencing a valid challenge
- **THEN** the row persists and is retrievable ordered by `ordinal`
