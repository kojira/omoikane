#!/usr/bin/env bash
# Create a Situation — the scenario-oriented reverse-lookup surface.
# A situation is a concrete, recurring SCENARIO ("when you are X-ing under Y")
# that binds the entries someone in that scenario needs together. This is
# DISTINCT from a UseCase (problem-KIND taxonomy): a situation is a
# task/context that usually spans several types and projects.
#
# Usage: post_situation.sh "<bilingual description>" [domain] [project_id]
#   The description should be query-shaped and bilingual (英日) so a person
#   IN that scenario recognises it. Returns the created situation JSON (id).
#
# Search existing situations FIRST (GET /v1/situations) and extend a matching
# one with link_situation.sh instead of creating a near-duplicate.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/load_env.sh"

DESC="${1:?bilingual scenario description required}"
DOMAIN="${2:-}"
PROJECT="${3:-}"

PAYLOAD=$(jq -n --arg d "$DESC" --arg dom "$DOMAIN" --arg pid "$PROJECT" '
  {description:$d}
  + (if $dom != "" then {domain:$dom} else {} end)
  + (if $pid != "" then {project_id:$pid} else {} end)')

curl --retry 5 --retry-connrefused -fsS -X POST "$KB_URL/v1/situations" \
    -H "Authorization: Bearer $KB_TOKEN" -H "Content-Type: application/json" \
    -d "$PAYLOAD"

curl --retry 5 --retry-connrefused -fsS -X POST "$KB_URL/v1/librarian/instances/$KB_INSTANCE_ID/heartbeat" \
    -H "Authorization: Bearer $KB_TOKEN" -H "Content-Type: application/json" \
    -d "$(jq -n --arg n "created situation: ${DESC:0:60}" '{note:$n, did_action:true}')" >/dev/null || true
