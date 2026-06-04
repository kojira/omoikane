-- UseCase as a first-class resource — see design.md §23.15.4.
--
-- Replaces the symptom/trigger PHRASE-per-entry model (003_reverse_index.sql)
-- as the primary reverse-lookup surface. The old tables stay for API back-
-- compat but are no longer the human-facing browse target.
--
-- One UseCase = one "kind of problem omoikane covers", with bilingual name
-- and description so the dashboard can switch ja/en. Many-to-many with
-- entries: one UseCase groups many entries; one entry can belong to many
-- UseCases. slug (kebab-case of name_en) is UNIQUE so parallel indexers
-- converge on the same row without coordination.

CREATE TABLE IF NOT EXISTS use_cases (
    id              TEXT PRIMARY KEY,        -- U-XXXXXX
    slug            TEXT NOT NULL UNIQUE,    -- 'mouth-articulation-weak'
    name_ja         TEXT NOT NULL,
    name_en         TEXT NOT NULL,
    description_ja  TEXT NOT NULL DEFAULT '',
    description_en  TEXT NOT NULL DEFAULT '',
    domain          TEXT,                    -- lipsync|audio|training|auth|web|...
    source          TEXT NOT NULL DEFAULT 'indexer',
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_use_cases_domain     ON use_cases(domain);
CREATE INDEX IF NOT EXISTS idx_use_cases_updated_at ON use_cases(updated_at DESC);

CREATE TABLE IF NOT EXISTS use_case_entries (
    use_case_id TEXT NOT NULL REFERENCES use_cases(id) ON DELETE CASCADE,
    entry_id    TEXT NOT NULL REFERENCES entries(id)   ON DELETE CASCADE,
    source      TEXT NOT NULL DEFAULT 'indexer',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (use_case_id, entry_id)
);
CREATE INDEX IF NOT EXISTS idx_uce_entry    ON use_case_entries(entry_id);
CREATE INDEX IF NOT EXISTS idx_uce_use_case ON use_case_entries(use_case_id);
