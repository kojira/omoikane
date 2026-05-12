-- migration: 009_auth
-- Phase A — Google OAuth + session token + extensibility for Phase B.
-- Phase B (separate migration 010) will add project_memberships and
-- email/password tables. These columns are present now but unused so
-- that Phase B is a code-only change.

-- users: add identity columns.
-- email + google_sub are UNIQUE-where-not-NULL via the partial indexes
-- below (SQLite doesn't enforce UNIQUE NULL the way Postgres does).
ALTER TABLE users ADD COLUMN email             TEXT;
ALTER TABLE users ADD COLUMN google_sub        TEXT;
ALTER TABLE users ADD COLUMN password_hash     TEXT;        -- bcrypt; Phase B
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMP;   -- Phase B
ALTER TABLE users ADD COLUMN avatar_url        TEXT;
ALTER TABLE users ADD COLUMN last_login_at     TIMESTAMP;

-- Enforce uniqueness only when the column is non-NULL. SQLite indexes
-- with WHERE clauses treat each NULL as distinct, so multiple rows can
-- have NULL email/sub during the migration window.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_email      ON users(email)      WHERE email      IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_google_sub ON users(google_sub) WHERE google_sub IS NOT NULL;

-- api_tokens: 'api' (long-lived agent / CLI) vs 'session' (browser, short).
-- Existing rows default to 'api' which matches their semantics.
ALTER TABLE api_tokens ADD COLUMN token_type TEXT NOT NULL DEFAULT 'api';
CREATE INDEX IF NOT EXISTS idx_api_tokens_type ON api_tokens(token_type);
