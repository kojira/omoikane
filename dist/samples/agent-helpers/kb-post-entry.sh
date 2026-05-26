#!/usr/bin/env bash
# kb-post-entry.sh — post one knowledge entry to omoikane safely.
# License: MIT. Sample only — copy and adapt to your runtime.
#
# Wraps POST /v1/entries with safe JSON construction (jq -n, never shell
# string interpolation), explicit HTTP-code handling, and unambiguous
# success / failure output. The dialog-sdk and lipsync agents both ran
# into "I built a huge curl one-liner and can't tell whether it
# worked" — this script removes that whole class of pain.
#
# === Quick start ===
#   export OMOIKANE_BASE_URL=https://kb.zenryoku.work
#   export OMOIKANE_API_KEY=<your bearer token>
#   ./kb-post-entry.sh \
#     --project lipsync \
#     --type trap \
#     --title "Realtime recording runner must not use AVFoundation full-screen capture" \
#     --symptom "ffmpeg AVFoundation video input captures screen 0 ..." \
#     --root-cause "ffmpeg avfoundation exposes whole-display inputs ..." \
#     --resolution "Use Playwright page recording or ScreenCaptureKit ..." \
#     --prohibited "Do not use ffmpeg -f avfoundation with Capture screen 0 ..." \
#     --tags "realtime,recording,privacy,screencapturekit,ffmpeg" \
#     --body-file ./body.md
#
# Output on success (printed to stdout, one line):
#   recorded: T-XXXXXX  https://kb.zenryoku.work/entries/T-XXXXXX
# Exit 0.
#
# Output on failure (printed to stderr, multi-line):
#   error: HTTP 400 BAD_REQUEST: <message>
#   <full response body>
# Exit non-zero.
#
# === Required env ===
#   OMOIKANE_BASE_URL  e.g. https://kb.zenryoku.work
#   OMOIKANE_API_KEY   bearer token (issued via invitation flow)
#
# === Required flags ===
#   --project <id>     existing project (e.g. lipsync, omoikane)
#   --type <type>      trap | lesson | decision | design | incident
#   --title <str>      one-line summary
#
# === Optional flags ===
#   --symptom <str>                ; --symptom-file <path>
#   --root-cause <str>             ; --root-cause-file <path>
#   --resolution <str>             ; --resolution-file <path>
#   --prohibited <str>             ; --prohibited-file <path>
#   --attempted-approaches <str>   ; --attempted-approaches-file <path>
#   --observed-behavior <str>      ; --observed-behavior-file <path>
#   --hypotheses <str>             ; --hypotheses-file <path>
#   --body <str>                   ; --body-file <path>      (markdown)
#   --status <s>                   default: ACTIVE (DRAFT also common)
#   --tags <csv>                   comma-separated, e.g. "ml,training,vgg"
#
# Any field can be passed inline (`--symptom "text"`) OR via file
# (`--symptom-file ./symptom.md`). File form is safer for long content
# that includes quotes, newlines, dollar signs, etc.
#
# === Why this script ===
# It does FOUR things you'd otherwise re-implement every call:
#   1. validates required args before any HTTP work
#   2. builds the JSON payload with jq -n so embedded text can contain
#      quotes / newlines / backslashes without shell heartbreak
#   3. uses `curl -sS -w '%{http_code}'` to capture the HTTP status and
#      treats anything other than 201 as failure (preserving the full
#      response body for diagnosis)
#   4. prints a single-line success record agents can grep / parse:
#      "recorded: <id>  <dashboard-url>"
#
# Anything not covered here you can do with raw curl per
# https://kb.zenryoku.work/skill.md — but for the common "post one
# entry, tell me it worked" path, this is the floor.

set -euo pipefail

# -------- Args ------------------------------------------------------

usage() {
    awk '/^# ===/{flag++; if (flag==4) exit} flag>=1' "$0" >&2
    exit 2
}

PROJECT=""; TYPE=""; TITLE=""; STATUS=""
SYMPTOM=""; ROOT_CAUSE=""; RESOLUTION=""; PROHIBITED=""
ATTEMPTED=""; OBSERVED=""; HYPOTHESES=""; BODY=""
TAGS_CSV=""

read_file_or_die() {
    local label="$1" path="$2"
    [[ -f "$path" ]] || { echo "error: $label-file: $path not found" >&2; exit 2; }
    cat "$path"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --project)              PROJECT="$2";   shift 2 ;;
        --type)                 TYPE="$2";      shift 2 ;;
        --title)                TITLE="$2";     shift 2 ;;
        --status)               STATUS="$2";    shift 2 ;;
        --symptom)              SYMPTOM="$2";   shift 2 ;;
        --symptom-file)         SYMPTOM=$(read_file_or_die symptom "$2"); shift 2 ;;
        --root-cause)           ROOT_CAUSE="$2"; shift 2 ;;
        --root-cause-file)      ROOT_CAUSE=$(read_file_or_die root-cause "$2"); shift 2 ;;
        --resolution)           RESOLUTION="$2"; shift 2 ;;
        --resolution-file)      RESOLUTION=$(read_file_or_die resolution "$2"); shift 2 ;;
        --prohibited)           PROHIBITED="$2"; shift 2 ;;
        --prohibited-file)      PROHIBITED=$(read_file_or_die prohibited "$2"); shift 2 ;;
        --attempted-approaches)      ATTEMPTED="$2"; shift 2 ;;
        --attempted-approaches-file) ATTEMPTED=$(read_file_or_die attempted "$2"); shift 2 ;;
        --observed-behavior)         OBSERVED="$2"; shift 2 ;;
        --observed-behavior-file)    OBSERVED=$(read_file_or_die observed "$2"); shift 2 ;;
        --hypotheses)           HYPOTHESES="$2"; shift 2 ;;
        --hypotheses-file)      HYPOTHESES=$(read_file_or_die hypotheses "$2"); shift 2 ;;
        --body)                 BODY="$2";      shift 2 ;;
        --body-file)            BODY=$(read_file_or_die body "$2"); shift 2 ;;
        --tags)                 TAGS_CSV="$2";  shift 2 ;;
        -h|--help)              usage ;;
        *)                      echo "unknown flag: $1" >&2; usage ;;
    esac
done

: "${OMOIKANE_BASE_URL:?OMOIKANE_BASE_URL env var is required}"
: "${OMOIKANE_API_KEY:?OMOIKANE_API_KEY env var is required}"

[[ -z "$PROJECT" ]] && { echo "error: --project is required" >&2; exit 2; }
[[ -z "$TYPE"    ]] && { echo "error: --type is required (trap|lesson|decision|design|incident)" >&2; exit 2; }
[[ -z "$TITLE"   ]] && { echo "error: --title is required" >&2; exit 2; }
# Server requires non-empty body. If the substance lives in
# symptom/root_cause/etc, pass `--body "see fields above"` (or any
# placeholder) — but the field must be set.
if [[ -z "$BODY" ]]; then
    echo "error: --body (or --body-file) is required — the server enforces a non-empty body. If the substance is captured by symptom/root_cause/etc., pass --body \"see fields above\" or similar." >&2
    exit 2
fi

case "$TYPE" in
    trap|lesson|decision|design|incident|librarian_meta|external_finding) ;;
    *) echo "error: --type must be one of trap|lesson|decision|design|incident (got: $TYPE)" >&2; exit 2 ;;
esac

# -------- Build payload --------------------------------------------

# Tags: split CSV into JSON array (jq handles the array build).
TAGS_JSON='[]'
if [[ -n "$TAGS_CSV" ]]; then
    TAGS_JSON=$(jq -nR --arg s "$TAGS_CSV" '$s | split(",") | map(. | gsub("^\\s+|\\s+$"; "")) | map(select(length > 0))')
fi

PAYLOAD=$(jq -n \
    --arg project    "$PROJECT" \
    --arg type       "$TYPE" \
    --arg title      "$TITLE" \
    --arg status     "${STATUS:-ACTIVE}" \
    --arg symptom    "$SYMPTOM" \
    --arg rootcause  "$ROOT_CAUSE" \
    --arg resolution "$RESOLUTION" \
    --arg prohibited "$PROHIBITED" \
    --arg attempted  "$ATTEMPTED" \
    --arg observed   "$OBSERVED" \
    --arg hypotheses "$HYPOTHESES" \
    --arg body       "$BODY" \
    --argjson tags   "$TAGS_JSON" \
    '{
        project_id: $project,
        type: $type,
        title: $title,
        status: $status,
        symptom: (if ($symptom | length) > 0 then $symptom else null end),
        root_cause: (if ($rootcause | length) > 0 then $rootcause else null end),
        resolution: (if ($resolution | length) > 0 then $resolution else null end),
        prohibited: (if ($prohibited | length) > 0 then $prohibited else null end),
        attempted_approaches: (if ($attempted | length) > 0 then $attempted else null end),
        observed_behavior: (if ($observed | length) > 0 then $observed else null end),
        hypotheses: (if ($hypotheses | length) > 0 then $hypotheses else null end),
        body: (if ($body | length) > 0 then $body else null end),
        body_format: (if ($body | length) > 0 then "markdown" else null end),
        tags: $tags
    } | with_entries(select(.value != null))')

# -------- POST -----------------------------------------------------

RAW=$(mktemp)
trap 'rm -f "$RAW"' EXIT

CODE=$(curl -sS -o "$RAW" -w '%{http_code}' \
    -X POST "$OMOIKANE_BASE_URL/v1/entries" \
    -H "Authorization: Bearer $OMOIKANE_API_KEY" \
    -H "Content-Type: application/json" \
    --data-binary @<(printf '%s' "$PAYLOAD"))

if [[ "$CODE" != "201" ]]; then
    echo "error: HTTP $CODE" >&2
    # Try to extract code+message from omoikane's error envelope.
    if jq -e '.error.code' "$RAW" >/dev/null 2>&1; then
        ec=$(jq -r '.error.code' "$RAW")
        em=$(jq -r '.error.message' "$RAW")
        echo "  $ec: $em" >&2
    fi
    echo "--- response body ---" >&2
    cat "$RAW" >&2
    echo >&2
    exit 1
fi

# -------- Success report -------------------------------------------

ID=$(jq -r '.id' "$RAW")
if [[ -z "$ID" || "$ID" == "null" ]]; then
    echo "error: 201 but missing id in response" >&2
    cat "$RAW" >&2
    exit 1
fi

echo "recorded: $ID  $OMOIKANE_BASE_URL/entries/$ID"
