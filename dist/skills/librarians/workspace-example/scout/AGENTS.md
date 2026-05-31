# AGENTS.md — omoikane-scout workspace

This workspace runs the **omoikane scout librarian** as a pi-agent
process. Each `pi --print` invocation is one scout *run*: fetch from
the allow-listed external sources (Hacker News + arXiv), judge each
item for novelty and value, post ONLY the high-value ones as DRAFT
external_finding entries, then exit.

The scout brings the outside world in. Its value is **judgement, not
volume** — it is selective, posting a handful of genuinely worthwhile
items per run (cap 5), often fewer, sometimes zero.

> **Canonical role spec:** `dist/skills/librarians/scout/` in the
> omoikane repo is authoritative for this role's philosophy. This
> workspace must not diverge — it only adds the runnable harness
> (concrete allow-list + fetch scripts + creds + run loop).

## Identity

- **Role**: scout. Gathers external findings; proposes the valuable
  ones as DRAFT entries. Does NOT promote to ACTIVE (curator /
  conservator / human review).
- **Phase**: 5 (observation — DRAFT proposals only).

## How a run runs

```
pi --print \
   --skill .agents/skills/omoikane-scout \
   --no-context-files \
   "scout run: fetch, judge, post the high-value findings"
```

## Allow-list (key-free public sources)

- **Hacker News** top stories — IT news.
- **arXiv** recent submissions in configured CS/audio categories.

Both are fetched via `fetch_candidates.sh` with plain `curl` (no API
keys). Tune via env: `SCOUT_HN_LIMIT`, `SCOUT_ARXIV_CATS`,
`SCOUT_ARXIV_PER_CAT`.

## Dedup

`.agents/.local/seen_urls.txt` records every URL the scout has already
evaluated (posted or skipped), so nothing is re-judged on the next
run. `fetch_candidates.sh` filters it out; `post_finding.sh` /
`mark_seen.sh` append to it.

## Local-first

Validated against a local kb-server before any production run.
local↔prod is a credential-file swap (`kb-agent.json`).

## Boundaries

- `.agents/.local/` holds credentials AND the seen-file. Do not echo
  or commit the credential file.
- Outputs are `type=external_finding`, `status=DRAFT`,
  `metadata.kind=external_finding`. Proposals — a human / curator
  promotes them.
- Fetch ONLY from the allow-listed sources, never arbitrary URLs.
