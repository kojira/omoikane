#!/usr/bin/env bash
# Link an entry to a Situation (a member of the scenario bundle).
# Idempotent — re-linking the same pair updates relevance/notes.
# Usage: link_situation.sh <situation_id> <entry_id> [relevance 0..1] [notes]
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/load_env.sh"

SIT="${1:?situation id required}"
ENTRY_ID="${2:?entry_id required}"
REL="${3:-0.8}"
NOTES="${4:-}"

PAYLOAD=$(jq -n --arg eid "$ENTRY_ID" --argjson rel "$REL" --arg n "$NOTES" '
  {entry_id:$eid, relevance:$rel} + (if $n != "" then {notes:$n} else {} end)')

curl --retry 5 --retry-connrefused -fsS -X POST "$KB_URL/v1/situations/$SIT/entries" \
    -H "Authorization: Bearer $KB_TOKEN" -H "Content-Type: application/json" \
    -d "$PAYLOAD"

curl --retry 5 --retry-connrefused -fsS -X POST "$KB_URL/v1/librarian/instances/$KB_INSTANCE_ID/heartbeat" \
    -H "Authorization: Bearer $KB_TOKEN" -H "Content-Type: application/json" \
    -d "$(jq -n --arg n "linked situation=$SIT entry=$ENTRY_ID" '{note:$n, did_action:true}')" >/dev/null || true
