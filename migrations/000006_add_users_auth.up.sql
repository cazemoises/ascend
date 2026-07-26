-- users(email, password_hash, timestamps) already exist since 000001.
-- Enforce case-insensitive email uniqueness for auth lookups.
CREATE UNIQUE INDEX idx_users_email_lower ON users (LOWER(email));
