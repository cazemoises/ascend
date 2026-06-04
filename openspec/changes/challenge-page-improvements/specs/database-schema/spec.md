## MODIFIED Requirements

### Requirement: challenges table stores problem definitions
The schema SHALL include a `challenges` table with: `id` (UUID primary key), `slug` (unique, not null), `title` (not null), `description` (text), `difficulty` (enum: easy/medium/hard), `time_limit_ms` (integer, NOT NULL, default 2000), `memory_limit_mb` (integer, NOT NULL, default 256), `notes` (TEXT, nullable — stores optional instructional text shown to the user on the challenge page), `created_at`, `updated_at`. The `time_limit_ms` and `memory_limit_mb` columns SHALL be added via migration 000002. The `notes` column SHALL be added via migration 000004 using `ALTER TABLE challenges ADD COLUMN notes TEXT` so the migration is safe on a non-empty table.

#### Scenario: Challenge row can be inserted
- **WHEN** a row is inserted into `challenges` with a unique slug
- **THEN** the row persists with the correct difficulty value

#### Scenario: time_limit_ms and memory_limit_mb default correctly
- **WHEN** a challenge row is inserted without specifying `time_limit_ms` or `memory_limit_mb`
- **THEN** the row persists with `time_limit_ms = 2000` and `memory_limit_mb = 256`

#### Scenario: Migration 000002 is safe on existing data
- **WHEN** migration 000002 is applied to a database that already has challenge rows
- **THEN** all existing rows gain the new columns with default values and no data is lost

#### Scenario: notes column is nullable
- **WHEN** a challenge row is inserted without specifying `notes`
- **THEN** the row persists with `notes = NULL`

#### Scenario: Migration 000004 is safe on existing data
- **WHEN** migration 000004 is applied to a database that already has challenge rows
- **THEN** all existing rows gain `notes = NULL` and no data is lost
