## MODIFIED Requirements

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
