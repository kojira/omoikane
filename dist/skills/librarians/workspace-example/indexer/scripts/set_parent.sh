#!/usr/bin/env bash
# set_parent.sh — repoint a UseCase under a parent (tidy mode).
#
# Usage: set_parent.sh <child_ref> <parent_ref>
#   <child_ref>  use_case id (U-XXXXXX) or slug
#   <parent_ref> use_case id or slug — pass empty string "" to un-root.
#
# Implementation: POST /v1/use_cases (upsert) with the child's existing
# slug + minimal name fields + the new parent_id. The server preserves
# everything else (description, domain, links to entries) — only parent
# changes.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/load_env.sh"

CHILD="${1:?child use_case ref (id or slug) required}"
PARENT="${2:-}"

# Resolve child to get its current name fields (POST upsert requires both).
CHILD_JSON=$(curl -fsS -H "Authorization: Bearer $KB_TOKEN" \
    "$KB_URL/v1/use_cases/$CHILD")
NAME_JA=$(jq -r '.use_case.name_ja' <<<"$CHILD_JSON")
NAME_EN=$(jq -r '.use_case.name_en' <<<"$CHILD_JSON")
SLUG=$(jq -r '.use_case.slug' <<<"$CHILD_JSON")

# If parent given as a slug or id, resolve to id (server upsert wants an id
# string in parent_id; slug also works for GET but we normalise here).
if [[ -n "$PARENT" ]]; then
    PARENT_ID=$(curl -fsS -H "Authorization: Bearer $KB_TOKEN" \
        "$KB_URL/v1/use_cases/$PARENT" | jq -r '.use_case.id')
else
    PARENT_ID=""
fi

PAYLOAD=$(jq -n --arg slug "$SLUG" --arg ja "$NAME_JA" --arg en "$NAME_EN" \
    --arg parent "$PARENT_ID" --arg src "indexer:${KB_INSTANCE_ID}" \
    '{slug:$slug, name_ja:$ja, name_en:$en, parent_id:$parent, source:$src}')

curl -fsS -X POST "$KB_URL/v1/use_cases" \
    -H "Authorization: Bearer $KB_TOKEN" -H "Content-Type: application/json" \
    -d "$PAYLOAD" | jq '{id, slug, parent_id}'
