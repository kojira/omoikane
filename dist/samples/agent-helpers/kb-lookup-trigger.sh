#!/usr/bin/env bash
# kb-lookup-trigger.sh — check omoikane BEFORE doing a sensitive operation.
# License: MIT. Sample only — copy and adapt to your runtime.
#
# Wraps POST /v1/lookup/by-trigger. The right time to call this is
# "before I start doing X" — if a hit's `prohibited` field matches what
# you were planning, ABORT and re-plan.
#
# === Quick start ===
#   export OMOIKANE_BASE_URL=https://kb.zenryoku.work
#   export OMOIKANE_API_KEY=<your bearer token>
#   ./kb-lookup-trigger.sh "kill llama-server with pkill"
#
# Output (one block per hit):
#   T-TORNYR  score=4.2  title: pkill -f <pattern> は無関係プロセスを巻き込む
#     prohibited: pkill -f <pattern> を確認なしに実行しない
#     resolution: pgrep -fl で対象を確認してから kill <pid> で個別に
#     https://kb.zenryoku.work/entries/T-TORNYR
#
# Exit 0 if 0+ hits. Exit non-zero only on transport / auth error.
#
# === Required env ===
#   OMOIKANE_BASE_URL
#   OMOIKANE_API_KEY
#
# === Args ===
#   $1   trigger description (required, free text)
#   --project <id>   optional, restrict to one project
#   --top <n>        optional, default 5
#   --include-prohibited     return full prohibited text (default: presence flag only)

set -euo pipefail

: "${OMOIKANE_BASE_URL:?OMOIKANE_BASE_URL env var is required}"
: "${OMOIKANE_API_KEY:?OMOIKANE_API_KEY env var is required}"

TRIGGER=""
PROJECT=""
TOP=5
INCLUDE_PROHIBITED=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --project) PROJECT="$2"; shift 2 ;;
        --top)     TOP="$2";     shift 2 ;;
        --include-prohibited) INCLUDE_PROHIBITED=true; shift ;;
        -h|--help)
            awk '/^# ===/{flag++; if (flag==4) exit} flag>=1' "$0" >&2
            exit 2 ;;
        *)
            if [[ -z "$TRIGGER" ]]; then TRIGGER="$1"
            else echo "unexpected arg: $1" >&2; exit 2
            fi
            shift ;;
    esac
done

[[ -z "$TRIGGER" ]] && { echo "error: trigger description is required as positional arg" >&2; exit 2; }

PAYLOAD=$(jq -n \
    --arg trigger "$TRIGGER" \
    --arg project "$PROJECT" \
    --argjson top "$TOP" \
    --argjson include "$INCLUDE_PROHIBITED" \
    '{
        trigger_description: $trigger,
        top_k: $top,
        include_prohibited: $include,
        project_id: (if ($project | length) > 0 then $project else null end)
    } | with_entries(select(.value != null))')

RAW=$(mktemp); trap 'rm -f "$RAW"' EXIT

CODE=$(curl -sS -o "$RAW" -w '%{http_code}' \
    -X POST "$OMOIKANE_BASE_URL/v1/lookup/by-trigger" \
    -H "Authorization: Bearer $OMOIKANE_API_KEY" \
    -H "Content-Type: application/json" \
    --data-binary @<(printf '%s' "$PAYLOAD"))

if [[ "$CODE" != "200" ]]; then
    echo "error: HTTP $CODE" >&2
    cat "$RAW" >&2; echo >&2
    exit 1
fi

# Pretty-print matches. Empty matches → print "no hits" and exit 0.
N=$(jq '.matches | length' "$RAW")
if [[ "$N" -eq 0 ]]; then
    echo "no hits for: $TRIGGER"
    exit 0
fi

# One block per match. Score formatted as %.1f.
jq -r --arg base "$OMOIKANE_BASE_URL" '
    .matches[] |
    "\(.entry_id)  score=\(.score | (.*10|floor)/10)  type=\(.type)  title: \(.title)" +
    (if .prohibited != "" then "\n  prohibited: \(.prohibited)" else "" end) +
    (if .resolution != "" then "\n  resolution: \(.resolution)" else "" end) +
    "\n  " + $base + "/entries/" + .entry_id + "\n"
' "$RAW"
