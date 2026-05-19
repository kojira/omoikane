-- migration: 018_librarian_progress
--
-- FIFO backlog tracking per librarian role.
--
-- Why this exists:
--   - The previous cataloger SKILL.md prescribed "process entries
--     created/updated in the last N hours". This has a bad failure mode:
--     entries that existed BEFORE the librarian came online (or were
--     created while it was down) are never processed. Old backlog
--     languishes forever.
--   - Correct model: pick the OLDEST unprocessed entry for this role
--     and process it. A row in librarian_progress means "this role has
--     looked at this entry and either acted or chose not to". Either
--     way the entry is no longer "unprocessed" for this role.
--   - One librarian can revisit an entry later (e.g. after enrichment
--     bump or after another librarian's proposal landed) by writing a
--     second progress row. We don't enforce uniqueness on (role,
--     entry_id) because "I looked at this again with new context" is
--     a valid recurring action.
--
-- Each librarian role maintains its own progress independently:
-- cataloger's processed set is disjoint from curator's processed set.
-- This means a fresh role can backfill from the beginning of the
-- corpus without affecting other roles' progress.
--
-- The `action` column is intentionally free text. Different roles
-- emit different actions (cataloger: summarized/tagged/reverse_indexed/
-- no_action; curator: status_proposed/supersede_proposed/...). The
-- API layer validates against the role's per-role vocabulary; the
-- DB stays permissive so adding actions doesn't require schema
-- migrations.

CREATE TABLE IF NOT EXISTS librarian_progress (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    role            TEXT    NOT NULL,          -- cataloger | curator | ...
    entry_id        TEXT    NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    instance_id     TEXT,                       -- which librarian instance did the work (FK soft)
    processed_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    action          TEXT    NOT NULL,          -- role-specific verb
    output_entry_id TEXT,                       -- produced librarian_meta DRAFT, if any
    notes           TEXT                        -- optional free-text rationale
);

-- The FIFO query is "find entries with no progress row for role X,
-- oldest first". An anti-join on (role, entry_id) drives this; the
-- composite index makes the join fast.
CREATE INDEX IF NOT EXISTS idx_librarian_progress_role_entry
    ON librarian_progress(role, entry_id);

-- For per-role audit / dashboard: "what has cataloger been doing?"
CREATE INDEX IF NOT EXISTS idx_librarian_progress_role_time
    ON librarian_progress(role, processed_at DESC);

-- For per-instance audit: "what has this specific librarian instance
-- done?"
CREATE INDEX IF NOT EXISTS idx_librarian_progress_instance
    ON librarian_progress(instance_id, processed_at DESC)
    WHERE instance_id IS NOT NULL;
