# AGENTS.md — omoikane-cataloger workspace

> **Canonical role spec:** `dist/skills/librarians/cataloger/` in the
> omoikane repo is authoritative for this role's philosophy (per-tick
> contract, owned domains, persona). This workspace must not diverge
> from it — it only adds the runnable harness (batch loop + scripts +
> creds).

This workspace exists to run the **omoikane cataloger librarian**
as a pi-agent process. Each `pi --print` invocation here is one
cataloger *session*: loop over the oldest unprocessed entries from
omoikane's backlog (up to 30 per session), and for each one decide
what (if anything) to write and post the result, then exit.

## Identity

- **Role**: cataloger (one of the 8 librarian roles in omoikane;
  see `~/develop/omoikane/dist/skills/librarians/cataloger/` for the
  reference bundle this workspace is derived from)
- **Domains owned**: tags, hierarchy, situations, plus the per-entry
  summary librarian_meta this workspace produces
- **Phase**: 5 (observation mode — proposals only, no destructive
  writes)
- **What this workspace is NOT**: a long-running daemon. Each session
  is a fresh `pi --print` invocation that drains a batch then exits.
  Scheduling (launchd / cron / a higher-level scheduler) lives
  outside the workspace.

## How a session runs

```
pi --print \
   --skill .agents/skills/omoikane-cataloger \
   --no-context-files \
   "drain a batch from the cataloger backlog"
```

The skill (`.agents/skills/omoikane-cataloger/SKILL.md`) tells the
LLM what to do with the bash tool. The credential file
(`.agents/.local/kb-agent.json`) is read by the helper scripts;
nothing in this AGENTS.md or the skill mentions specific paths to
secrets.

## How to operate

- One batch per session (up to 30 entries), then exit. The scheduler
  decides when the next session fires.
- Heartbeat is posted as part of every entry processed (success OR
  no_action). If the scheduler skips sessions, the missed heartbeats
  will show up as a liveness alarm on the coordinator dashboard.
- Emergency stop: if `GET /v1/librarian/instances/<id>.status` is
  `stopped`, each loop iteration still heartbeats but does NOT act.
  This is honored automatically by `scripts/backlog_next.sh`.

## Boundaries

- This workspace's `.agents/.local/` is the only place credentials
  live. Do not echo them, do not commit them, do not copy them
  outside.
- The local `data/kb.db` is a snapshot of production for offline
  validation. It is overwritten freely; do not assume it is canonical.
- Production writes are gated by which credential file is in
  `.agents/.local/kb-agent.json`. Switching between local and
  production is a credential-file swap, not a skill change.
