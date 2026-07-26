CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE difficulty AS ENUM ('easy', 'medium', 'hard');
CREATE TYPE submission_status AS ENUM ('pending', 'running', 'accepted', 'wrong_answer', 'error');

CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE challenges (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT        NOT NULL UNIQUE,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    difficulty  difficulty  NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE submissions (
    id           UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID              NOT NULL REFERENCES users(id),
    challenge_id UUID              NOT NULL REFERENCES challenges(id),
    language     TEXT              NOT NULL,
    code         TEXT              NOT NULL,
    status       submission_status NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMPTZ       NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ       NOT NULL DEFAULT NOW()
);

CREATE TABLE test_cases (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    challenge_id    UUID        NOT NULL REFERENCES challenges(id),
    input           TEXT        NOT NULL DEFAULT '',
    expected_output TEXT        NOT NULL DEFAULT '',
    is_sample       BOOLEAN     NOT NULL DEFAULT false,
    ordinal         INTEGER     NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_test_cases_challenge_ordinal ON test_cases(challenge_id, ordinal);
