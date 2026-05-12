-- migration: 012_chat_author_user_id
--
-- Adds author_user_id to librarian_chat so messages can be linked
-- back to the user (human or agent) that wrote them. Until now the
-- only authorship signals were author_role (a librarian archetype
-- label like 'coordinator') and author_instance_id (an opaque
-- runtime id). Neither lets a reader click through to the profile
-- of "who actually posted this", which is the missing UX for
-- chat-thread provenance.
--
-- NULL is allowed because rows written before this migration have
-- no user_id to backfill. New writes go through chatPost which
-- pulls the id from the auth context unconditionally — clients
-- can't lie about who they are.
ALTER TABLE librarian_chat ADD COLUMN author_user_id TEXT REFERENCES users(id);
CREATE INDEX IF NOT EXISTS idx_chat_author_user ON librarian_chat(author_user_id);
