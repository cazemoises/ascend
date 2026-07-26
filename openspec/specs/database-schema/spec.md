# Spec: Database Schema

## Purpose

The database schema defines the persistent data model for the Ascend platform. It is managed exclusively through versioned migration files to ensure reproducibility and auditability. The database is PostgreSQL.

## Requirements

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
The schema SHALL include a `challenges` table with: `id` (UUID primary key), `slug` (unique, not null), `title` (not null), `description` (text), `difficulty` (enum: easy/medium/hard), `time_limit_ms` (integer, NOT NULL, default 2000), `memory_limit_mb` (integer, NOT NULL, default 256), `created_at`, `updated_at`. The `time_limit_ms` and `memory_limit_mb` columns SHALL be added via migration 000002 (separate from the initial schema), using `ALTER TABLE` with defaults so the migration is safe on a non-empty table.

#### Scenario: Challenge row can be inserted
- **WHEN** a row is inserted into `challenges` with a unique slug
- **THEN** the row persists with the correct difficulty value

#### Scenario: time_limit_ms and memory_limit_mb default correctly
- **WHEN** a challenge row is inserted without specifying `time_limit_ms` or `memory_limit_mb`
- **THEN** the row persists with `time_limit_ms = 2000` and `memory_limit_mb = 256`

#### Scenario: Migration 000002 is safe on existing data
- **WHEN** migration 000002 is applied to a database that already has challenge rows
- **THEN** all existing rows gain the new columns with default values and no data is lost

### Requirement: submissions table stores code submissions
The schema SHALL include a `submissions` table with: `id` (UUID primary key, default `gen_random_uuid()`), `challenge_id` (FK → challenges, not null), `language` (TEXT, not null), `source_code` (TEXT, not null), `status` (TEXT, NOT NULL, default `'pending'`), `created_at` (TIMESTAMPTZ, not null, default `now()`), `updated_at` (TIMESTAMPTZ, not null, default `now()`). This table SHALL be created by migration `000003_submissions`. The `status` column SHALL use `TEXT` (not a PostgreSQL enum) so the judge worker can write additional status values (`running`, `accepted`, `wrong_answer`, `time_limit_exceeded`, `runtime_error`, `compile_error`) without requiring a schema migration. There is no `user_id` column until authentication is implemented.

#### Scenario: Submission row linked to challenge
- **WHEN** a submission is inserted referencing a valid challenge ID
- **THEN** the row persists with `status = 'pending'` and the correct `language` and `source_code`

#### Scenario: Submission with invalid challenge_id is rejected
- **WHEN** a submission is inserted with a non-existent `challenge_id`
- **THEN** the database returns a foreign key constraint violation

#### Scenario: Default status is pending
- **WHEN** a submission row is inserted without specifying `status`
- **THEN** the row persists with `status = 'pending'`

### Requirement: test_cases table stores per-challenge test data
The schema SHALL include a `test_cases` table with: `id` (UUID primary key), `challenge_id` (FK → challenges), `input` (text), `expected_output` (text), `is_sample` (boolean), `ordinal` (integer for ordering).

#### Scenario: Test case linked to challenge
- **WHEN** a test case is inserted referencing a valid challenge
- **THEN** the row persists and is retrievable ordered by `ordinal`
