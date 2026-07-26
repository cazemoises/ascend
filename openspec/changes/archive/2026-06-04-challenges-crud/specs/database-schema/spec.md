## MODIFIED Requirements

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
