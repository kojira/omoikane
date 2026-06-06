-- UseCase tree: parent_id makes use_cases a self-referential tree.
-- See design.md §23.15.4. The design intent (kojira-san, 2026-06-06) is
-- BOTTOM-UP growth: leaves are stable; when the count of top-level rows
-- (parent_id IS NULL) grows past a threshold, the indexer extracts a
-- handful of META categories ABOVE them and rewrites their parent_id to
-- point at the new meta. The same rule runs recursively at any level —
-- 1段でも何段でも同じロジックで育つ。
--
-- ON DELETE SET NULL: deleting a meta category un-roots its children
-- rather than cascading; archiving a meta should not delete the leaves
-- that were beneath it.

ALTER TABLE use_cases ADD COLUMN parent_id TEXT REFERENCES use_cases(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_use_cases_parent ON use_cases(parent_id);
