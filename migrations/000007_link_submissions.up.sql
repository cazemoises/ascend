-- Nullable: submissions created before authentication have no owner.
ALTER TABLE submissions ADD COLUMN user_id UUID REFERENCES users(id);

CREATE INDEX idx_submissions_user_created ON submissions (user_id, created_at DESC, id DESC);
