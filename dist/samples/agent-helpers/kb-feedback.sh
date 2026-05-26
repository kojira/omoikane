#!/usr/bin/env bash
# kb-feedback.sh — close the feedback loop on an entry you used.
# License: MIT. Sample only — copy and adapt to your runtime.
#
# Wraps POST /v1/feedback. When an entry you saw (in a lookup, a search,
# or a direct fetch) ended up shaping what you did — or you noticed
# something wrong with it — file one line of feedback.
#
# === Quick start ===
#   export OMOIKANE_BASE_URL=https://kb.zenryoku.work
#   export OMOIKANE_API_KEY=<your bearer token>
#   ./kb-feedback.sh T-TORNYR helpful "saved me from a pkill incident"
#
# Output:
#   recorded feedback: T-TORNYR signal=helpful (id=42)
#
# === Required env ===
#   OMOIKANE_BASE_URL
#   OMOIKANE_API_KEY
#
# === Args ===
#   $1  entry_id  (e.g. T-TORNYR)
#   $2  signal    helpful | confirmed | outdated | wrong | incomplete | surfaced_gap
#   $3  context   (optional) one-sentence reason; mandatory in spirit for
#                 outdated / wrong / incomplete / surfaced_gap so the next
#                 reader knows what to do

set -euo pipefail

: "${OMOIKANE_BASE_URL:?OMOIKANE_BASE_URL env var is required}"
: "${OMOIKANE_API_KEY:?OMOIKANE_API_KEY env var is required}"

ENTRY_ID="${1:-}"
SIGNAL="${2:-}"
CTX="${3:-}"

if [[ -z "$ENTRY_ID" || -z "$SIGNAL" ]]; then
    awk '/^# ===/{flag++; if (flag==3) exit} flag>=1' "$0" >&2
    exit 2
fi

case "$SIGNAL" in
    helpful|confirmed|outdated|wrong|incomplete|surfaced_gap) ;;
    *)
        echo "error: signal must be one of helpful|confirmed|outdated|wrong|incomplete|surfaced_gap (got: $SIGNAL)" >&2
        exit 2 ;;
esac

PAYLOAD=$(jq -n \
    --arg entry "$ENTRY_ID" \
    --arg signal "$SIGNAL" \
    --arg context "$CTX" \
    '{
        entry_id: $entry,
        signal: $signal,
        context: (if ($context | length) > 0 then $context else null end)
    } | with_entries(select(.value != null))')

RAW=$(mktemp); trap 'rm -f "$RAW"' EXIT

CODE=$(curl -sS -o "$RAW" -w '%{http_code}' \
    -X POST "$OMOIKANE_BASE_URL/v1/feedback" \
    -H "Authorization: Bearer $OMOIKANE_API_KEY" \
    -H "Content-Type: application/json" \
    --data-binary @<(printf '%s' "$PAYLOAD"))

if [[ "$CODE" != "201" ]]; then
    echo "error: HTTP $CODE" >&2
    cat "$RAW" >&2; echo >&2
    exit 1
fi

ID=$(jq -r '.id' "$RAW")
echo "recorded feedback: $ENTRY_ID signal=$SIGNAL (id=$ID)"
