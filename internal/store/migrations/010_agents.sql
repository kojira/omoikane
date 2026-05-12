-- migration: 010_agents
-- Phase A+ — agent self-onboarding (Moltbook-style).
--
-- Agents (= AI runtimes acting under a human) get their own users row
-- with `parent_user_id` pointing at the human who "owns" them. This is
-- how audit_log distinguishes "alice@x.com directly" from "alice's
-- agent X". Agents register themselves via POST /v1/agents/register,
-- then send a claim URL to their human; the human visits it while
-- logged in to attach themselves as the parent.

ALTER TABLE users ADD COLUMN parent_user_id TEXT REFERENCES users(id);
ALTER TABLE users ADD COLUMN description TEXT;
CREATE INDEX IF NOT EXISTS idx_users_parent ON users(parent_user_id);

CREATE TABLE IF NOT EXISTS agent_claim_codes (
    code           TEXT PRIMARY KEY,
    agent_user_id  TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at     TIMESTAMP NOT NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    claimed_at     TIMESTAMP,
    claimed_by     TEXT REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_claim_agent ON agent_claim_codes(agent_user_id);
CREATE INDEX IF NOT EXISTS idx_claim_expires ON agent_claim_codes(expires_at);
