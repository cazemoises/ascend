CREATE TYPE submission_status AS ENUM ('pending', 'running', 'accepted', 'wrong_answer', 'error');

ALTER TABLE submissions RENAME COLUMN source_code TO code;

ALTER TABLE submissions ALTER COLUMN status DROP DEFAULT;
ALTER TABLE submissions ALTER COLUMN status TYPE submission_status USING status::submission_status;
ALTER TABLE submissions ALTER COLUMN status SET DEFAULT 'pending';

-- user_id cannot be restored; add as nullable
ALTER TABLE submissions ADD COLUMN user_id UUID REFERENCES users(id);
