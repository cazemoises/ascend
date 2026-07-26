## ADDED Requirements

### Requirement: User can submit code for a challenge
The system SHALL expose `POST /api/v1/challenges/{id}/submissions` that accepts a `language` and `source_code`, persists a submission record with `status: pending`, enqueues a job to the Redis `submissions` list, and returns `202 Accepted` with the `submission_id`.

Supported languages are: `go`, `python`, `javascript`.

#### Scenario: Valid submission is accepted
- **WHEN** a POST request is sent with a valid `language` and non-empty `source_code` for an existing challenge
- **THEN** the system responds with 202 and a JSON body containing `submission_id`

#### Scenario: Submission is persisted with pending status
- **WHEN** a valid submission request is processed
- **THEN** a row exists in `submissions` with `status = 'pending'` and the correct `challenge_id` and `language`

#### Scenario: Submission job is enqueued to Redis
- **WHEN** a valid submission is accepted
- **THEN** a JSON message `{"submission_id":"<uuid>","challenge_id":"<uuid>"}` is pushed to the Redis list `submissions`

#### Scenario: Unknown challenge returns 404
- **WHEN** a POST request is sent with a `challenge_id` that does not exist
- **THEN** the system responds with 404

#### Scenario: Unsupported language returns 422
- **WHEN** a POST request is sent with a `language` not in `[go, python, javascript]`
- **THEN** the system responds with 422

#### Scenario: Missing source_code returns 422
- **WHEN** a POST request is sent with an empty or absent `source_code`
- **THEN** the system responds with 422

#### Scenario: Missing language returns 422
- **WHEN** a POST request is sent without a `language` field
- **THEN** the system responds with 422

#### Scenario: Invalid JSON body returns 400
- **WHEN** a POST request is sent with malformed JSON
- **THEN** the system responds with 400
