-- AI Content Optimization — persistence schema (Milestone 18)
--
-- This is the append-only SQLite DDL that a production Repository adapter
-- implements. The shipped adapter is an in-memory MemoryRepository
-- (internal/contentoptimizer/store.go) that satisfies the same Repository port
-- (internal/contentoptimizer/ports.go), so swapping in SQLite requires no
-- changes to the optimizer, pattern, trend, recommendation, prompt, or reasoning
-- engines.
--
-- Core invariant: optimization history is IMMUTABLE and APPEND-ONLY. Reports,
-- recommendations, patterns, trends, prompt versions, and approvals are never
-- overwritten. A prompt is NEVER auto-replaced: a new version is a new row, and
-- adoption is recorded as an approval — rollback is a read of an earlier version.
--
-- Portable to Postgres with minimal changes (INTEGER->BIGINT, REAL->DOUBLE
-- PRECISION, TEXT timestamps -> timestamptz, JSON columns -> jsonb).

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

-- One immutable optimization run.
CREATE TABLE IF NOT EXISTS optimization_reports (
    run_id            TEXT PRIMARY KEY,             -- deterministic run id (idempotency key)
    schema_version    TEXT NOT NULL,
    as_of             TEXT NOT NULL,                -- analytics window this run covers
    executive_summary TEXT NOT NULL,
    confidence        REAL NOT NULL,                -- overall run confidence (0–1)
    sufficiency       REAL NOT NULL,                -- data-quality 0–100
    document_json     TEXT NOT NULL,                -- full serialized report
    generated_at      TEXT NOT NULL
);

-- Recommendation history (append-only; every recommendation cites its evidence).
CREATE TABLE IF NOT EXISTS recommendations (
    id              TEXT NOT NULL,
    run_id          TEXT NOT NULL REFERENCES optimization_reports(run_id),
    category        TEXT NOT NULL,                  -- content-strategy | publishing-strategy | seo | ...
    title           TEXT NOT NULL,
    detail          TEXT NOT NULL,
    rationale       TEXT NOT NULL,
    evidence_json   TEXT NOT NULL DEFAULT '[]',     -- the analytics that ground it
    confidence      REAL NOT NULL,
    expected_impact TEXT NOT NULL,                  -- low | medium | high
    priority        INTEGER NOT NULL,
    pattern_id      TEXT,
    PRIMARY KEY (run_id, id)
);
CREATE INDEX IF NOT EXISTS idx_recs_category ON recommendations (category);

-- Pattern library (append-only). Winning/losing signal chains with evidence.
CREATE TABLE IF NOT EXISTS patterns (
    id            TEXT PRIMARY KEY,
    kind          TEXT NOT NULL,                    -- winning | losing
    name          TEXT NOT NULL,
    signal_chain_json TEXT NOT NULL,                -- ["High CTR","Strong hook",...]
    description   TEXT NOT NULL,
    support       INTEGER NOT NULL,
    confidence    REAL NOT NULL,
    evidence_json TEXT NOT NULL DEFAULT '[]',
    examples_json TEXT NOT NULL DEFAULT '[]',
    first_seen    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_patterns_kind ON patterns (kind);

-- Trend history (append-only), one row per (run, horizon).
CREATE TABLE IF NOT EXISTS trends (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id      TEXT NOT NULL REFERENCES optimization_reports(run_id),
    horizon     TEXT NOT NULL,                      -- daily | weekly | monthly | quarterly | long-term
    emerging_json TEXT NOT NULL DEFAULT '[]',
    declining_json TEXT NOT NULL DEFAULT '[]',
    windows_json  TEXT NOT NULL DEFAULT '[]',
    fatigue_json  TEXT NOT NULL DEFAULT '[]',
    created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_trends_run ON trends (run_id);

-- Prompt version repository (append-only; rollback = read an earlier version).
CREATE TABLE IF NOT EXISTS prompt_versions (
    template_id   TEXT NOT NULL,
    version       INTEGER NOT NULL,
    body          TEXT NOT NULL,
    notes         TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL,                    -- draft | proposed | approved | rejected | retired
    -- Measured outcomes for this version:
    usage_count       INTEGER NOT NULL DEFAULT 0,
    avg_ctr           REAL NOT NULL DEFAULT 0,
    avg_engagement    REAL NOT NULL DEFAULT 0,
    avg_watch_minutes REAL NOT NULL DEFAULT 0,
    avg_reading_min   REAL NOT NULL DEFAULT 0,
    avg_score         REAL NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL,
    superseded_by INTEGER,
    PRIMARY KEY (template_id, version)              -- immutable per (template, version)
);

-- Time-series of prompt-version measured outcomes (never overwritten).
CREATE TABLE IF NOT EXISTS prompt_metrics (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    template_id       TEXT NOT NULL,
    version           INTEGER NOT NULL,
    as_of             TEXT NOT NULL,
    usage_count       INTEGER NOT NULL DEFAULT 0,
    avg_ctr           REAL NOT NULL DEFAULT 0,
    avg_engagement    REAL NOT NULL DEFAULT 0,
    avg_watch_minutes REAL NOT NULL DEFAULT 0,
    avg_reading_min   REAL NOT NULL DEFAULT 0,
    avg_score         REAL NOT NULL DEFAULT 0,
    UNIQUE (template_id, version, as_of)
);

-- Human approvals audit (append-only). The latest decision per (template,
-- version) resolves a version's EFFECTIVE status; the version row is never mutated.
CREATE TABLE IF NOT EXISTS approvals (
    id         TEXT PRIMARY KEY,
    kind       TEXT NOT NULL,                       -- prompt | recommendation | usage
    target_id  TEXT NOT NULL,                       -- template id or recommendation id
    version    INTEGER,
    status     TEXT NOT NULL,                       -- approved | rejected | ...
    decider    TEXT NOT NULL,
    reason     TEXT,
    decided_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_approvals_target ON approvals (kind, target_id, version);

-- Confidence-score history for reproducibility audits.
CREATE TABLE IF NOT EXISTS confidence_scores (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id     TEXT NOT NULL REFERENCES optimization_reports(run_id),
    subject    TEXT NOT NULL,                       -- pattern id / recommendation id / 'overall'
    confidence REAL NOT NULL,
    recorded_at TEXT NOT NULL
);

-- Rollback pattern (adopt an earlier prompt version without deleting history):
--   INSERT INTO approvals (id, kind, target_id, version, status, decider, decided_at)
--   VALUES (?, 'prompt', ?, <older_version>, 'approved', ?, ?);
-- The optimizer's ActiveVersion() then resolves to that version on next read.
