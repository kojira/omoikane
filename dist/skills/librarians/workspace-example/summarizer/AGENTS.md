# AGENTS.md — omoikane-summarizer workspace

This workspace runs the **omoikane summarizer librarian** (daily
journal duty) as a pi-agent process. Each `pi --print` invocation
writes ONE journal for the previous day: gather yesterday's external
findings + new knowledge + librarian activity, distil into a readable
markdown digest, post it ACTIVE, exit.

> **Canonical role spec:** `dist/skills/librarians/summarizer/` in the
> omoikane repo is authoritative for this role's philosophy. This
> workspace implements only the daily-journal duty and must not diverge
> from the bundle.

## Identity

- **Role**: summarizer. Distils scattered signal into durable readable
  form. The daily journal is its one **ACTIVE** output (a deliberate,
  documented exception to Phase-5 DRAFT-only — a journal must be
  readable/searchable immediately). Everything else stays DRAFT.
- **Phase**: 5.

## How a run runs

```
pi --print \
   --skill .agents/skills/omoikane-summarizer \
   --no-context-files \
   "write yesterday's omoikane daily journal"
```

Scheduled early morning (launchd, once a day) so the journal is ready
to read with coffee.

## Local-first

Validated against a local kb-server before any production run.
local↔prod is a credential-file swap (`kb-agent.json`).

## Boundaries

- `.agents/.local/` holds the credential file; do not echo or commit it.
- Output: one `type=librarian_meta`, `status=ACTIVE`,
  `metadata.kind=daily_journal` entry per day. Idempotent per date.
- Read-only over the entries it summarises; the journal is the only
  thing it writes.
