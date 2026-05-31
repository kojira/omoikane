# AGENTS.md — omoikane-detective workspace

This workspace runs the **omoikane detective librarian** as a
pi-agent process. Each `pi --print` invocation is one detective
*session*: loop over backlog entries (up to 15 per session), and for
each one search for semantic near-duplicates, judge them, and — when
a true duplicate exists — post a duplicate-finding DRAFT (a proposal),
then exit.

The detective exists because the server's own clustering is lexical
(Jaccard on symptom tokens) and cannot see synonyms, paraphrase, or
**cross-language** duplicates. The semantic judgement is the LLM's
job; the server only supplies cheap candidates.

> **Canonical role spec:** `dist/skills/librarians/detective/` in the
> omoikane repo is authoritative for this role's philosophy. This
> workspace must not diverge from it — it only adds the runnable
> harness (scripts + batch loop + creds).

## Identity

- **Role**: detective (one of the 8 librarian roles). Discovers
  duplicates/conflicts/lineage and PROPOSES relation edges; does NOT
  resolve them (that is curator's / a human's job). Propose only
  when you can cite a concrete shared claim, mechanism, or lineage —
  see SKILL.md "Your job" for the bar.
- **Phase**: 5 (observation — proposals only, no destructive writes).
  The detective never creates relations, supersedes, merges, edits,
  or deletes. It only writes DRAFT findings.
- **What this workspace is NOT**: a long-running daemon. Each session
  is a fresh `pi --print` that examines a batch then exits.
  Scheduling lives outside the workspace.

## How a session runs

```
pi --print \
   --skill .agents/skills/omoikane-detective \
   --no-context-files \
   "find duplicates: examine a batch from the detective backlog"
```

The skill (`.agents/skills/omoikane-detective/SKILL.md`) drives the
LLM. The credential file (`.agents/.local/kb-agent.json`) is read by
the helper scripts; nothing here names secret paths.

## Local-first

This workspace is validated against a **local kb-server**
(`http://localhost:8095`, prod DB snapshot) before any production
run. Switching local↔prod is a credential-file swap
(`kb-agent.json`), not a skill change — exactly like the cataloger.

## How to operate

- One batch per session (up to 15 entries), then exit.
- Heartbeat + progress are recorded per examined entry (flagged or
  no_action) by the helper scripts.
- Emergency stop: if the instance status is `stopped`, each loop
  iteration heartbeats but does not act (honored by backlog_next.sh).

## Boundaries

- `.agents/.local/` is the only place credentials live. Do not echo,
  commit, or copy them out.
- Findings are `type=librarian_meta`, `status=DRAFT`,
  `metadata.kind=duplicate_finding`. They are PROPOSALS — a curator
  or human decides whether to create the relation / merge / supersede.
