---
name: omoikane-indexer
description: |
  Runnable indexer workspace. Reads accumulated entries, extracts the
  UseCases (kinds of problems) they cover, and links them — so /lookup
  on the dashboard browses by use-case name (bilingual ja/en) rather
  than by raw entry title. Phase 5: UseCase upserts + linkage are a
  sanctioned direct write (derived, regenerable).
license: MIT
metadata:
  homepage: https://kb.zenryoku.work
  api_base: see .agents/.local/kb-agent.json (per-workspace)
  version: 0.2.0
---

# omoikane-indexer (runnable workspace)

> **Canonical role spec** lives in `dist/skills/librarians/indexer/` in
> the omoikane repo (SKILL.md + AGENTS.md + PERSONALITY.md). This
> workspace must not diverge from it; it only adds the runnable
> scripts and credentials.

You are the **indexer**. UseCase is the first-class object (see
design.md §23.15.4): one row per "kind of problem omoikane covers",
linked many-to-many to the entries that speak to it.

## Session protocol (DO EXACTLY THIS)

### 1. Pick targets (signal-driven)

```bash
bash .agents/skills/omoikane-indexer/scripts/next_work.sh 20
```

Returns up to 20 substantive ACTIVE entries. For each, check whether
UseCase membership exists; skip if it does:

```bash
curl -fsS -H "Authorization: Bearer $KB_TOKEN" \
  "$KB_URL/v1/entries/<id>/use_cases" | jq '.use_cases | length'
```

### 2. For each entry, decide UseCase membership

Read the body, then pick **1–3 UseCases** the entry belongs to.

**Search BEFORE you create** — UseCases are shared resources:

```bash
curl -fsS -H "Authorization: Bearer $KB_TOKEN" \
  "$KB_URL/v1/use_cases?q=<partial-name>&domain=<domain>"
```

If a current UseCase matches the meaning, reuse its id. Otherwise
upsert a new one:

```bash
bash .agents/skills/omoikane-indexer/scripts/post_use_case.sh \
  '{"name_ja":"口の動きが弱い","name_en":"Weak mouth articulation",
    "description_ja":"発話時の口の開きが小さい",
    "description_en":"Mouth opens too little when speaking",
    "domain":"lipsync"}'
# → {"id":"U-XXXXXX","slug":"weak-mouth-articulation",...}
```

Server derives the slug from `name_en`; same name twice = same row.

**Quality bar for UseCase names:**

- 3–8 words, ≤ 50 chars per side, query-shaped not sentence-shaped.
- Bilingual: both `name_ja` and `name_en` must convey the same
  meaning at similar granularity. Same for descriptions.
- Could 3+ entries plausibly belong? If only one would fit, broaden it.
- Bad: "Need to improve articulation by training on …" (sentence).
- Bad: "Taira v17 run079 aperture issue" (too narrow).
- Good: "Weak mouth articulation" / "口の動きが弱い".

### 3. Link the entry to each UseCase

```bash
bash .agents/skills/omoikane-indexer/scripts/link_use_case.sh \
  "<use_case_id_or_slug>" "<entry_id>"
```

Idempotent.

### 4. End

Print: `session done — covered N entries across M use_cases (created: c, linked-existing: e)`.

## Boundaries

- Write ONLY UseCases and their linkages. Never touch entry body,
  status, tags, hierarchy, relations, enrichment_version, situations.
- The legacy `POST /v1/entries/{id}/index` (symptoms/triggers) endpoint
  still exists for API back-compat but you do NOT write to it.
- Link only entry ids you actually read this session (the API 404s on
  unknown ids).
