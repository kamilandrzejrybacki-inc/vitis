-- Clank session store schema. Applied programmatically by postgres_store.go New().

CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    duration_ms BIGINT,
    exit_code INT,
    parser_confidence DOUBLE PRECISION,
    observation_confidence DOUBLE PRECISION,
    auth_mode TEXT NOT NULL DEFAULT 'unknown',
    blocked_reason TEXT,
    bytes_captured BIGINT,
    warnings JSONB NOT NULL DEFAULT '[]'::jsonb,
    terminal_cols INT,
    terminal_rows INT
);

CREATE TABLE IF NOT EXISTS turns (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    turn_index INT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS turns_session_turn_index_unique
ON turns(session_id, turn_index);

CREATE TABLE IF NOT EXISTS stream_events (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    timestamp TIMESTAMPTZ NOT NULL,
    kind TEXT NOT NULL,
    chunk_raw BYTEA NOT NULL,
    chunk_text TEXT,
    chunk_encoding TEXT NOT NULL DEFAULT 'raw'
);

CREATE INDEX IF NOT EXISTS stream_events_session_timestamp_index
ON stream_events(session_id, timestamp);
