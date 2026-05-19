#!/usr/bin/env bash
# Sample cataloger tick — demonstrates the full per-tick flow that
# pi-agent (or any LLM runtime) would drive.
#
# This script does NOT invoke an LLM. It demonstrates the omoikane
# API calls a librarian runtime makes around the LLM call:
#
#   1. Heartbeat-register and resolve own instance_id
#   2. Check emergency-stop status
#   3. Pull oldest unprocessed entry for role=cataloger
#   4. (LLM call goes here — runtime-specific)
#   5. Post a librarian_meta DRAFT for the summary
#   6. Record progress so the entry is not re-processed
#   7. Heartbeat and exit
#
# Steps 5 and 6 here use a stub summary so the script is testable
# without an LLM. Replace step 5's body with whatever your runtime
# produces.
#
# Usage:
#   export KB_URL=https://kb.zenryoku.work
#   export KB_TOKEN=<librarian-scoped token from invite redemption>
#   export INSTANCE_ID=<your instance id, from /v1/librarian/instances>
#   ./sample-cataloger-tick.sh
#
# Environment is the contract. Paths to local config files and the
# choice of LLM runtime stay outside this script per L-ES3SMD.

set -euo pipefail

: "${KB_URL:?KB_URL required (e.g. https://kb.zenryoku.work)}"
: "${KB_TOKEN:?KB_TOKEN required (librarian-scoped Bearer token)}"
: "${INSTANCE_ID:?INSTANCE_ID required (your registered librarian instance id)}"

ROLE=cataloger
AUTH_HEADER="Authorization: Bearer ${KB_TOKEN}"
CT_JSON="Content-Type: application/json"

note() { printf '▶ %s\n' "$*" >&2; }

# --- 1. Emergency-stop check -------------------------------------------------

note "checking emergency-stop status"
status=$(curl -fsS -H "${AUTH_HEADER}" \
  "${KB_URL}/v1/librarian/instances/${INSTANCE_ID}" | jq -r .status)
if [[ "${status}" == "stopped" ]]; then
    note "instance is in emergency stop; heartbeat-only and exit"
    curl -fsS -X POST -H "${AUTH_HEADER}" -H "${CT_JSON}" \
        -d '{"note":"honoring emergency stop"}' \
        "${KB_URL}/v1/librarian/instances/${INSTANCE_ID}/heartbeat" >/dev/null
    exit 0
fi

# --- 2. Pull oldest unprocessed entry ---------------------------------------

note "pulling oldest unprocessed entry for role=${ROLE}"
backlog_response=$(curl -sS -H "${AUTH_HEADER}" \
  "${KB_URL}/v1/librarian/backlog/next?role=${ROLE}")
http_code=$?

# If the response has an error envelope with code NOT_FOUND, backlog is empty.
err_code=$(echo "${backlog_response}" | jq -r '.error.code // empty')
if [[ "${err_code}" == "NOT_FOUND" ]]; then
    note "backlog drained; heartbeat and exit"
    curl -fsS -X POST -H "${AUTH_HEADER}" -H "${CT_JSON}" \
        -d '{"note":"backlog drained","did_action":false}' \
        "${KB_URL}/v1/librarian/instances/${INSTANCE_ID}/heartbeat" >/dev/null
    exit 0
fi

entry=$(echo "${backlog_response}" | jq .entry)
entry_id=$(echo "${entry}" | jq -r .id)
entry_title=$(echo "${entry}" | jq -r .title)
backlog_size=$(echo "${backlog_response}" | jq -r .backlog_size)
note "got entry ${entry_id} (${entry_title}); backlog size = ${backlog_size}"

# --- 3. LLM call would go here ----------------------------------------------
#
# Real runtime (pi-agent / Claude Code / OpenCode / etc.) calls an LLM
# with:
#   - SKILL.md (this librarian's bundle)
#   - AGENTS.md (per-tick decision protocol)
#   - PERSONALITY.md (persona, voice, biases)
#   - the source entry JSON above
#
# It expects back a structured response naming one of:
#   - summarized (produce a summary librarian_meta)
#   - tagged (propose retag/hierarchy/situation moves)
#   - reverse_indexed (group N similar entries into a navigation entry)
#   - no_action (record and move on)
#
# For this sample we use a stub summary body so the flow is testable
# without a live LLM.

summary_body=$(cat <<EOF
# Summary of $(echo "${entry_title}" | sed 's/"/\\"/g')

## Subject
Stub summary produced by the sample cataloger driver. Replace step 3 of
sample-cataloger-tick.sh with a real LLM call to generate this body.

## Core claim
Placeholder — the live cataloger reads the source entry and writes 2–3
sentences capturing what an agent would learn from it.

## When to retrieve
sample, stub, placeholder, cataloger demo

## Domain
sample

## Caveats
This is a stub. Do not rely on the summary body for real retrieval.

## Source
- entry_id: ${entry_id}
- generated_by: sample-cataloger-tick.sh
EOF
)

# --- 4. Post the librarian_meta DRAFT ---------------------------------------

note "posting librarian_meta DRAFT for ${entry_id}"
draft_payload=$(jq -n \
  --arg title "Summary: ${entry_title}" \
  --arg body "${summary_body}" \
  --arg source_id "${entry_id}" \
  --arg instance "${INSTANCE_ID}" \
  '{
    project_id: "omoikane",
    type: "librarian_meta",
    status: "DRAFT",
    title: $title,
    body: $body,
    body_format: "markdown",
    tags: ["librarian","cataloger","summary"],
    metadata: ({
      role: "cataloger",
      instance_id: $instance,
      kind: "cataloger_summary",
      source_entry_id: $source_id
    } | tostring)
  }')

draft_response=$(curl -fsS -X POST -H "${AUTH_HEADER}" -H "${CT_JSON}" \
    -d "${draft_payload}" "${KB_URL}/v1/entries")
draft_id=$(echo "${draft_response}" | jq -r .id)
note "DRAFT id = ${draft_id}"

# --- 5. Record progress -----------------------------------------------------

note "recording progress for ${entry_id}"
curl -fsS -X POST -H "${AUTH_HEADER}" -H "${CT_JSON}" \
  -d "$(jq -n \
        --arg role "${ROLE}" \
        --arg entry "${entry_id}" \
        --arg out "${draft_id}" \
        --arg instance "${INSTANCE_ID}" \
        '{role:$role, entry_id:$entry, instance_id:$instance,
          action:"summarized", output_entry_id:$out,
          notes:"sample driver stub summary"}')" \
  "${KB_URL}/v1/librarian/progress" | jq -c '{id, role, action}'

# --- 6. Heartbeat -----------------------------------------------------------

note "heartbeating"
curl -fsS -X POST -H "${AUTH_HEADER}" -H "${CT_JSON}" \
  -d "$(jq -n --arg note "processed ${entry_id} -> ${draft_id}" \
        '{note:$note, did_action:true}')" \
  "${KB_URL}/v1/librarian/instances/${INSTANCE_ID}/heartbeat" >/dev/null

note "tick complete"
