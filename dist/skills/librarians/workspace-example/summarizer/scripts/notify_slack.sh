#!/usr/bin/env bash
# Post the day's daily journal to Slack via an incoming webhook.
# Called by the wrapper AFTER the journal is written, so Slack delivery is
# deterministic (not left to the LLM). Safe to run even with no webhook
# configured — it just skips.
#
# Webhook URL is a SECRET: read from .agents/.local/slack-webhook.json
#   { "webhook_url": "https://hooks.slack.com/services/..." }
# (that dir is gitignored). Falls back to $SLACK_WEBHOOK_URL.
#
# Usage: notify_slack.sh [YYYY-MM-DD]   (default: yesterday JST = the day the
#        journal covers)
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/load_env.sh"

LOCAL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.local" && pwd)"
WEBHOOK=""
if [ -f "$LOCAL_DIR/slack-webhook.json" ]; then
    WEBHOOK=$(jq -r '.webhook_url // empty' "$LOCAL_DIR/slack-webhook.json" 2>/dev/null || true)
fi
WEBHOOK="${WEBHOOK:-${SLACK_WEBHOOK_URL:-}}"
if [ -z "$WEBHOOK" ]; then
    echo "[slack] no webhook configured (.agents/.local/slack-webhook.json) — skipping"
    exit 0
fi

TARGET="${1:-$(TZ=Asia/Tokyo date -v-1d +%Y-%m-%d 2>/dev/null || TZ=Asia/Tokyo date -d 'yesterday' +%Y-%m-%d)}"

# Find the daily_journal entry for TARGET.
RESP=$(curl --retry 5 --retry-connrefused -fsS -H "Authorization: Bearer $KB_TOKEN" \
    "$KB_URL/v1/entries?type=librarian_meta&limit=80")

PAYLOAD=$(KB_URL="$KB_URL" TARGET="$TARGET" python3 - "$RESP" <<'PY'
import os, sys, json
target = os.environ["TARGET"]; kb = os.environ["KB_URL"].rstrip("/")
data = json.loads(sys.argv[1], strict=False)
j = None
for e in data.get("entries", []):
    m = e.get("metadata") or {}
    if not isinstance(m, dict):
        continue
    if m.get("kind") == "daily_journal" and m.get("journal_date") == target:
        j = e; break
if j is None:
    print(""); sys.exit(0)

body = j.get("body") or ""
# Markdown -> Slack mrkdwn: headers to bold, strip stray markers, trim.
lines = []
for ln in body.split("\n"):
    s = ln.rstrip()
    if s.startswith("#"):
        s = "*" + s.lstrip("# ").strip() + "*"
    lines.append(s)
text = "\n".join(lines).strip()
LIMIT = 3500
if len(text) > LIMIT:
    text = text[:LIMIT].rsplit("\n", 1)[0] + "\n…（続き）"

url = f"{kb}/entries/{j['id']}"
header = f"📝 *omoikane 日次ジャーナル {target}*"
full = f"{header}\n\n{text}\n\n全文 → {url}"
print(json.dumps({"text": full, "unfurl_links": False}, ensure_ascii=False))
PY
)

if [ -z "$PAYLOAD" ]; then
    echo "[slack] no daily_journal for $TARGET — skipping"
    exit 0
fi

CODE=$(curl --retry 5 --retry-connrefused -sS -o /dev/null -w "%{http_code}" \
    -X POST "$WEBHOOK" -H "Content-Type: application/json" -d "$PAYLOAD")
echo "[slack] posted journal $TARGET → HTTP $CODE"
