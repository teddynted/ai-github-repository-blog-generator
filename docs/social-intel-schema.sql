-- Social Media Intelligence — persistence schema (Milestone 16)
--
-- This is the SQLite DDL that a production Repository adapter implements. The
-- shipped adapter is an in-memory MemoryRepository (internal/socialintel/store.go)
-- that satisfies the same Repository port (internal/socialintel/ports.go), so
-- swapping in SQLite requires no changes to the collector, analytics, or
-- reporting engines.
--
-- Core invariant: snapshots are IMMUTABLE and APPEND-ONLY. Historical metrics
-- are never overwritten. Idempotency is enforced by a UNIQUE key on
-- (platform, date) for accounts and (platform, content_id, date) for content —
-- a second write for the same day is rejected, not updated. Use
-- `INSERT ... ON CONFLICT DO NOTHING` to make daily collection re-runnable.
--
-- Portable to Postgres with minimal changes (INTEGER->BIGINT, REAL->DOUBLE
-- PRECISION, TEXT timestamps -> timestamptz).

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

-- One immutable daily snapshot of an account's aggregate metrics.
CREATE TABLE IF NOT EXISTS account_snapshots (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    platform           TEXT    NOT NULL,           -- YouTube | Instagram | X | TikTok | ...
    snapshot_date      TEXT    NOT NULL,           -- ISO date YYYY-MM-DD (collection granularity)
    followers          INTEGER NOT NULL DEFAULT 0, -- subscribers / followers / fans
    following          INTEGER NOT NULL DEFAULT 0,
    views              INTEGER NOT NULL DEFAULT 0,
    impressions        INTEGER NOT NULL DEFAULT 0,
    reach              INTEGER NOT NULL DEFAULT 0,
    profile_visits     INTEGER NOT NULL DEFAULT 0,
    watch_time_minutes INTEGER NOT NULL DEFAULT 0,
    extra_json         TEXT    NOT NULL DEFAULT '{}', -- platform-specific metrics (saves, revenue, ...)
    collected_at       TEXT    NOT NULL,             -- RFC3339 ingest timestamp
    UNIQUE (platform, snapshot_date)                 -- dedupe: one snapshot per platform per day
);

CREATE INDEX IF NOT EXISTS idx_account_platform_date
    ON account_snapshots (platform, snapshot_date);

-- One immutable daily snapshot of a single piece of content. Unifies YouTube
-- videos, Instagram posts/reels, X posts, and TikTok videos via `kind`. The same
-- content_id appears once per snapshot_date, forming a per-item time series used
-- for evergreen / retention analysis.
CREATE TABLE IF NOT EXISTS content_snapshots (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    platform           TEXT    NOT NULL,
    content_id         TEXT    NOT NULL,
    kind               TEXT    NOT NULL,           -- video | short | reel | post | thread
    title              TEXT    NOT NULL DEFAULT '',
    published_at       TEXT    NOT NULL DEFAULT '', -- ISO date the content went live
    snapshot_date      TEXT    NOT NULL,           -- ISO date of this measurement
    views              INTEGER NOT NULL DEFAULT 0,
    likes              INTEGER NOT NULL DEFAULT 0,
    comments           INTEGER NOT NULL DEFAULT 0,
    shares             INTEGER NOT NULL DEFAULT 0,
    saves              INTEGER NOT NULL DEFAULT 0,
    impressions        INTEGER NOT NULL DEFAULT 0,
    ctr                REAL    NOT NULL DEFAULT 0,
    avg_view_seconds   REAL    NOT NULL DEFAULT 0,
    retention_pct      REAL    NOT NULL DEFAULT 0,
    completion_rate    REAL    NOT NULL DEFAULT 0,
    watch_time_minutes INTEGER NOT NULL DEFAULT 0,
    subscribers_gained INTEGER NOT NULL DEFAULT 0,
    subscribers_lost   INTEGER NOT NULL DEFAULT 0,
    tags_json          TEXT    NOT NULL DEFAULT '[]',
    extra_json         TEXT    NOT NULL DEFAULT '{}',
    collected_at       TEXT    NOT NULL,
    UNIQUE (platform, content_id, snapshot_date)   -- dedupe: one measurement per item per day
);

CREATE INDEX IF NOT EXISTS idx_content_platform_date
    ON content_snapshots (platform, snapshot_date);
CREATE INDEX IF NOT EXISTS idx_content_history
    ON content_snapshots (platform, content_id, snapshot_date);

-- Optional: cache generated morning briefings / advisory packages for audit and
-- fast retrieval. The intelligence is always re-derivable from the snapshots
-- above; this table is a convenience, not a source of truth.
CREATE TABLE IF NOT EXISTS intelligence_reports (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    report_date    TEXT NOT NULL,                  -- ISO date the report covers
    kind           TEXT NOT NULL,                  -- briefing | advisory
    schema_version TEXT NOT NULL,                  -- e.g. 1.0.0
    document_json  TEXT NOT NULL,                  -- full serialized report
    generated_at   TEXT NOT NULL,
    UNIQUE (report_date, kind)
);

-- Optional: API-usage audit log (rate-limit / quota / rotation forensics).
-- Never store secret values here — only which credential *name* was used.
CREATE TABLE IF NOT EXISTS api_audit_log (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    platform      TEXT NOT NULL,
    endpoint      TEXT NOT NULL,
    credential    TEXT NOT NULL,                   -- env var NAME only, e.g. YOUTUBE_ACCESS_TOKEN
    status_code   INTEGER,
    error_code    TEXT,                            -- auth | rate_limit | quota | platform_down | ...
    recoverable   INTEGER NOT NULL DEFAULT 0,
    occurred_at   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_platform_time
    ON api_audit_log (platform, occurred_at);

-- Idempotent daily collection pattern:
--   INSERT INTO account_snapshots (platform, snapshot_date, followers, ..., collected_at)
--   VALUES (?, ?, ?, ..., ?)
--   ON CONFLICT (platform, snapshot_date) DO NOTHING;
