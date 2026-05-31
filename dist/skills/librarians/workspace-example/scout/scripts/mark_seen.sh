#!/usr/bin/env bash
# Mark one or more candidate URLs as seen WITHOUT posting — used for
# low-value items the scout evaluated and decided to skip, so they are
# never re-evaluated on the next run.
#
# Backed by the SQLite seen-store (seen_store.py), which scales to 100k+
# URLs with indexed lookups.
#
# Usage: mark_seen.sh <url> [<url> ...]
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ "$#" -ge 1 ] || { echo "usage: mark_seen.sh <url> [<url> ...]" >&2; exit 2; }
python3 "$SCRIPT_DIR/seen_store.py" add skipped "$@"
