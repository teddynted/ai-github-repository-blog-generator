-- Content Analytics & Performance — persistence schema (Milestone 17)
--
-- This is the normalized SQLite DDL that a production Repository adapter
-- implements. The shipped adapter is an in-memory MemoryRepository
-- (internal/contentanalytics/store.go) that satisfies the same Repository port
-- (internal/contentanalytics/ports.go), so swapping in SQLite requires no
-- changes to the collector, trend, report, dashboard, or optimization engines.
--
-- Core invariant: metrics snapshots are IMMUTABLE and APPEND-ONLY. Performance
-- history is never overwritten. Idempotency is enforced by a UNIQUE key on
-- (publication_id, period, snapshot_date). Use `INSERT ... ON CONFLICT DO NOTHING`
-- so daily collection is safely re-runnable.
--
-- Portable to Postgres with minimal changes (INTEGER->BIGINT, REAL->DOUBLE
-- PRECISION, TEXT timestamps -> timestamptz).

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

-- The published artifacts under measurement. This is the CURRENT record for a
-- publication (its facts — title, url, status — may change); it is not history.
-- id/content_id are shared with internal/publishing so distribution and
-- analytics join without coupling.
CREATE TABLE IF NOT EXISTS publications (
    id            TEXT PRIMARY KEY,             -- analytics publication id (== publishing id)
    content_id    TEXT NOT NULL,
    platform      TEXT NOT NULL,                -- Dev.to | Medium | Hashnode | YouTube | ...
    content_type  TEXT NOT NULL,                -- Technical Blog | YouTube Video | ...
    title         TEXT NOT NULL DEFAULT '',
    url           TEXT NOT NULL DEFAULT '',
    platform_id   TEXT NOT NULL DEFAULT '',     -- platform-side id (video id, article id)
    author        TEXT NOT NULL DEFAULT '',
    tags_json     TEXT NOT NULL DEFAULT '[]',
    keywords_json TEXT NOT NULL DEFAULT '[]',
    status        TEXT NOT NULL DEFAULT 'published', -- published | scheduled | draft | removed
    scheduled     INTEGER NOT NULL DEFAULT 0,
    published_at  TEXT,                         -- RFC3339
    last_updated  TEXT
);

CREATE INDEX IF NOT EXISTS idx_publications_platform ON publications (platform);

-- One immutable, normalized metrics snapshot of a publication at a period+date.
-- period = daily | weekly | monthly. Weekly/monthly rows are roll-ups keyed by
-- the bucket start (Monday / first-of-month). The reach/engagement/watch/click/
-- growth families below can also be split into their own tables (schema shown)
-- for storage engines that prefer full normalization; the single-table form is
-- the default and matches the in-memory adapter.
CREATE TABLE IF NOT EXISTS analytics_snapshots (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    publication_id TEXT NOT NULL REFERENCES publications(id),
    content_id     TEXT NOT NULL,
    platform       TEXT NOT NULL,
    period         TEXT NOT NULL,                -- daily | weekly | monthly
    snapshot_date  TEXT NOT NULL,                -- ISO date (bucket start for weekly/monthly)

    -- Reach
    views          INTEGER NOT NULL DEFAULT 0,
    impressions    INTEGER NOT NULL DEFAULT 0,
    unique_viewers INTEGER NOT NULL DEFAULT 0,
    audience_reach INTEGER NOT NULL DEFAULT 0,

    -- Engagement
    likes     INTEGER NOT NULL DEFAULT 0,
    reactions INTEGER NOT NULL DEFAULT 0,
    comments  INTEGER NOT NULL DEFAULT 0,
    replies   INTEGER NOT NULL DEFAULT 0,
    shares    INTEGER NOT NULL DEFAULT 0,
    reposts   INTEGER NOT NULL DEFAULT 0,
    saves     INTEGER NOT NULL DEFAULT 0,
    bookmarks INTEGER NOT NULL DEFAULT 0,

    -- Watch
    watch_time_minutes INTEGER NOT NULL DEFAULT 0,
    avg_view_seconds   REAL    NOT NULL DEFAULT 0,
    retention_pct      REAL    NOT NULL DEFAULT 0,
    completion_rate    REAL    NOT NULL DEFAULT 0,

    -- Click
    ctr             REAL    NOT NULL DEFAULT 0,
    thumbnail_ctr   REAL    NOT NULL DEFAULT 0,
    link_clicks     INTEGER NOT NULL DEFAULT 0,
    external_clicks INTEGER NOT NULL DEFAULT 0,

    -- Growth
    subscribers_gained INTEGER NOT NULL DEFAULT 0,
    followers_gained   INTEGER NOT NULL DEFAULT 0,
    subscribers_lost   INTEGER NOT NULL DEFAULT 0,
    audience_growth    INTEGER NOT NULL DEFAULT 0,

    partial      INTEGER NOT NULL DEFAULT 0,     -- some fields could not be collected
    collected_at TEXT NOT NULL,                  -- RFC3339 ingest timestamp
    UNIQUE (publication_id, period, snapshot_date) -- immutability / dedupe key
);

CREATE INDEX IF NOT EXISTS idx_snap_pub_period ON analytics_snapshots (publication_id, period, snapshot_date);
CREATE INDEX IF NOT EXISTS idx_snap_period_date ON analytics_snapshots (period, snapshot_date);

-- Optional fully-normalized breakout tables. A storage engine may use these
-- instead of the wide analytics_snapshots columns; each row 1:1 with a snapshot.
CREATE TABLE IF NOT EXISTS platform_metrics (
    snapshot_id INTEGER PRIMARY KEY REFERENCES analytics_snapshots(id),
    views INTEGER, impressions INTEGER, unique_viewers INTEGER, audience_reach INTEGER
);
CREATE TABLE IF NOT EXISTS engagement_metrics (
    snapshot_id INTEGER PRIMARY KEY REFERENCES analytics_snapshots(id),
    likes INTEGER, reactions INTEGER, comments INTEGER, replies INTEGER,
    shares INTEGER, reposts INTEGER, saves INTEGER, bookmarks INTEGER
);
CREATE TABLE IF NOT EXISTS watch_metrics (
    snapshot_id INTEGER PRIMARY KEY REFERENCES analytics_snapshots(id),
    watch_time_minutes INTEGER, avg_view_seconds REAL, retention_pct REAL, completion_rate REAL
);

-- Where views came from, per snapshot (search / feed / external / ...).
CREATE TABLE IF NOT EXISTS traffic_sources (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_id INTEGER NOT NULL REFERENCES analytics_snapshots(id),
    source      TEXT NOT NULL,
    views       INTEGER NOT NULL DEFAULT 0,
    percent     REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_traffic_snapshot ON traffic_sources (snapshot_id);

-- Audience growth attributable to a publication over a period (subscriber view).
CREATE TABLE IF NOT EXISTS subscribers (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    publication_id     TEXT NOT NULL REFERENCES publications(id),
    period             TEXT NOT NULL,
    snapshot_date      TEXT NOT NULL,
    subscribers_gained INTEGER NOT NULL DEFAULT 0,
    subscribers_lost   INTEGER NOT NULL DEFAULT 0,
    UNIQUE (publication_id, period, snapshot_date)
);

-- Precomputed trend rows (day/week/month-over-month) for fast dashboard reads.
-- Always re-derivable from analytics_snapshots; this table is a cache.
CREATE TABLE IF NOT EXISTS trends (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    platform       TEXT NOT NULL,
    publication_id TEXT,                         -- NULL for platform-aggregate trends
    metric         TEXT NOT NULL,                -- views | engagements | ctr | ...
    as_of_date     TEXT NOT NULL,
    current_value  REAL NOT NULL,
    dod_pct        REAL, wow_pct REAL, mom_pct REAL,
    direction      TEXT,                         -- up | down | flat
    UNIQUE (platform, publication_id, metric, as_of_date)
);

-- Optional: API-usage audit log (rate-limit / quota / auth / rotation forensics).
-- Never store secret values here — only which credential *name* was used.
CREATE TABLE IF NOT EXISTS api_audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    platform    TEXT NOT NULL,
    endpoint    TEXT NOT NULL,
    credential  TEXT NOT NULL,                   -- env var NAME only, e.g. YOUTUBE_API_KEY
    status_code INTEGER,
    error_code  TEXT,                            -- auth | rate_limit | quota | server | not_found
    recoverable INTEGER NOT NULL DEFAULT 0,
    occurred_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_platform_time ON api_audit_log (platform, occurred_at);

-- Idempotent daily collection pattern:
--   INSERT INTO analytics_snapshots (publication_id, content_id, platform, period, snapshot_date, views, ..., collected_at)
--   VALUES (?, ?, ?, 'daily', ?, ?, ..., ?)
--   ON CONFLICT (publication_id, period, snapshot_date) DO NOTHING;
