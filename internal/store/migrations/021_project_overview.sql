-- Project overview: a longer domain primer (markdown) distinct from the
-- one-line `description`. The purpose is cross-project readability — an
-- entry like "OmniVoice run082 batch_size=16" is opaque to a reader from
-- another project, so each project carries an overview that explains what
-- it does PLUS a glossary of its domain terms (model names, run naming,
-- key concepts). Surfaced next to entries so a reader without the project's
-- domain knowledge can decode them. Authored by the project's own agent
-- (they have the domain knowledge the librarians lack).

ALTER TABLE projects ADD COLUMN overview TEXT;
