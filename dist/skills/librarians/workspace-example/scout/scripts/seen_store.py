#!/usr/bin/env python3
"""SQLite-backed archive of every external item the scout has evaluated.

Originally just a dedup ledger of URLs; now a small asset — for each
item we keep url, title, body (abstract/summary where the source gives
one), lang (cheap heuristic), pubdate, source, and the action taken
(posted | skipped | migrated). URL is the PRIMARY KEY, so dedup stays an
O(log M) indexed lookup even at 100k+ rows.

DB: <workspace>/.agents/.local/seen.db (or $SCOUT_SEEN_DB).
Run cache: $SCOUT_CANDIDATES_CACHE (default .local/last_candidates.json) —
`filter` writes the fresh, lang-tagged candidates there so `add` can
enrich a URL into a full row without the caller re-supplying metadata.

Subcommands:
  filter   read JSON array of {source,url,title,body,pubdate} from stdin;
           tag each with lang, drop already-seen URLs, write the fresh
           ones to the run cache, and print them as a JSON array.
  add <action> <url> [<url> ...]
           record URLs as seen with `action`, enriching each from the
           run cache (title/body/lang/pubdate/source). Idempotent.
  count    print number of rows.
  recent [N]  print the N most recent rows as JSON (diagnostics / asset peek).
"""
import json
import os
import re
import sqlite3
import sys

COLUMNS = ["url", "title", "body", "lang", "pubdate", "source", "action"]

_KANA = re.compile(r"[぀-ヿ]")          # hiragana / katakana → ja
_HANGUL = re.compile(r"[가-힣]")          # → ko
_CJK = re.compile(r"[一-鿿]")             # ideographs (no kana) → zh


def detect_lang(text):
    t = text or ""
    if _KANA.search(t):
        return "ja"
    if _HANGUL.search(t):
        return "ko"
    if _CJK.search(t):
        return "zh"
    return "en"


def _db_path():
    if os.environ.get("SCOUT_SEEN_DB"):
        return os.environ["SCOUT_SEEN_DB"]
    local = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                         "..", "..", "..", ".local")
    return os.path.normpath(os.path.join(local, "seen.db"))


def _cache_path():
    if os.environ.get("SCOUT_CANDIDATES_CACHE"):
        return os.environ["SCOUT_CANDIDATES_CACHE"]
    return os.path.join(os.path.dirname(_db_path()), "last_candidates.json")


def _connect():
    path = _db_path()
    conn = sqlite3.connect(path)
    conn.execute("""
        CREATE TABLE IF NOT EXISTS seen (
            url        TEXT PRIMARY KEY,
            title      TEXT,
            body       TEXT,
            lang       TEXT,
            pubdate    TEXT,
            source     TEXT,
            action     TEXT,
            first_seen TEXT DEFAULT (datetime('now'))
        )""")
    # Upgrade an older (url, action, first_seen) table in place.
    existing = {r[1] for r in conn.execute("PRAGMA table_info(seen)")}
    for col in ("title", "body", "lang", "pubdate", "source"):
        if col not in existing:
            conn.execute(f"ALTER TABLE seen ADD COLUMN {col} TEXT")
    # One-time import of a legacy flat seen_urls.txt next to the DB.
    legacy = os.path.join(os.path.dirname(path), "seen_urls.txt")
    if os.path.exists(legacy):
        with open(legacy) as f:
            urls = [ln.strip() for ln in f if ln.strip()]
        conn.executemany(
            "INSERT OR IGNORE INTO seen(url, action) VALUES (?, 'migrated')",
            [(u,) for u in urls])
        os.rename(legacy, legacy + ".imported")
    conn.commit()
    return conn


def cmd_filter(conn):
    items = json.load(sys.stdin)
    if not isinstance(items, list):
        items = []
    cur = conn.cursor()
    seen_urls, fresh = set(), []
    for it in items:
        url = (it or {}).get("url")
        if not url or url in seen_urls:
            continue
        seen_urls.add(url)
        it["lang"] = detect_lang((it.get("title", "") + " " + it.get("body", "")))
        if cur.execute("SELECT 1 FROM seen WHERE url=? LIMIT 1", (url,)).fetchone() is None:
            fresh.append(it)
    with open(_cache_path(), "w") as f:
        json.dump(fresh, f, ensure_ascii=False)
    json.dump(fresh, sys.stdout, ensure_ascii=False)


def _load_cache():
    try:
        with open(_cache_path()) as f:
            return {c["url"]: c for c in json.load(f) if c.get("url")}
    except (OSError, ValueError):
        return {}


def cmd_add(conn, action, urls):
    cache = _load_cache()
    rows = []
    for u in urls:
        if not u:
            continue
        c = cache.get(u, {})
        rows.append((u, c.get("title", ""), c.get("body", ""), c.get("lang", ""),
                     c.get("pubdate", ""), c.get("source", ""), action))
    conn.executemany(
        "INSERT OR IGNORE INTO seen(url,title,body,lang,pubdate,source,action) "
        "VALUES (?,?,?,?,?,?,?)", rows)
    conn.commit()
    print(f"recorded {len(rows)} url(s) as '{action}'")


def cmd_count(conn):
    print(conn.execute("SELECT COUNT(*) FROM seen").fetchone()[0])


def cmd_recent(conn, n):
    cur = conn.execute(
        "SELECT url,title,lang,pubdate,source,action,first_seen "
        "FROM seen ORDER BY first_seen DESC LIMIT ?", (n,))
    cols = [d[0] for d in cur.description]
    json.dump([dict(zip(cols, r)) for r in cur.fetchall()], sys.stdout, ensure_ascii=False)


def main():
    if len(sys.argv) < 2:
        print("usage: seen_store.py {filter|add <action> <url>...|count|recent [N]}", file=sys.stderr)
        sys.exit(2)
    conn = _connect()
    sub = sys.argv[1]
    if sub == "filter":
        cmd_filter(conn)
    elif sub == "add":
        if len(sys.argv) < 3:
            print("add requires <action> and at least one url", file=sys.stderr)
            sys.exit(2)
        cmd_add(conn, sys.argv[2], sys.argv[3:])
    elif sub == "count":
        cmd_count(conn)
    elif sub == "recent":
        cmd_recent(conn, int(sys.argv[2]) if len(sys.argv) > 2 else 20)
    else:
        print(f"unknown subcommand: {sub}", file=sys.stderr)
        sys.exit(2)


if __name__ == "__main__":
    main()
