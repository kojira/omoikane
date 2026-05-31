# AGENTS.md — omoikane-curator workspace

This workspace runs the **omoikane curator librarian** as a pi-agent
process. Each `pi --print` invocation is one curator *session*: loop
over backlog entries (up to 15 per session); for each, classify it
(detective relation proposal / status-health case / not-mine), and
either propose a resolution or record no_action, then exit.

The curator closes the dedup loop: detective discovers and proposes
relations as DRAFTs; curator verifies them from the entries and
proposes the resolution (canonical pick + supersede, synthesize,
coexist, or reject). Both stay in Phase-5 DRAFT space — a human or
Phase-6 actor executes.

> **Canonical role spec:** `dist/skills/librarians/curator/` in the
> omoikane repo is authoritative for this role's philosophy. This
> workspace must not diverge — it only adds the runnable harness
> (batch loop + scripts + creds).

## Identity

- **Role**: curator. Resolves relations detective discovers; owns
  status lifecycle and supersede *proposals*. Does NOT discover
  relations (detective) or mutate anything (Phase 5).
- **Phase**: 5 (observation — DRAFT proposals only).

## How a session runs

```
pi --print \
   --skill .agents/skills/omoikane-curator \
   --no-context-files \
   "resolve a batch from the curator backlog"
```

## Local-first

Validated against a local kb-server (prod DB snapshot) before any
production run. local↔prod is a credential-file swap (`kb-agent.json`).

## Boundaries

- `.agents/.local/` is the only place credentials live. Do not echo,
  commit, or copy them out.
- Outputs are `type=librarian_meta`, `status=DRAFT`,
  `metadata.kind=curator_resolution`. PROPOSALS — a human / Phase-6
  actor decides whether to execute the supersede / status change.
