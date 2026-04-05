package postgres

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

type Store struct {
	pool *pgxpool.Pool
}

const migrationSQL = `
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
`

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	if _, err := pool.Exec(ctx, migrationSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) CreateSession(session model.Session) error {
	_, err := s.pool.Exec(context.Background(), `
INSERT INTO sessions (
    session_id, provider, status, started_at, ended_at, duration_ms, exit_code,
    parser_confidence, observation_confidence, auth_mode, blocked_reason, bytes_captured,
    warnings, terminal_cols, terminal_rows
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
`,
		session.ID, session.Provider, string(session.Status), session.StartedAt, session.EndedAt, session.DurationMs, session.ExitCode,
		session.ParserConfidence, session.ObservationConfidence, session.AuthMode, session.BlockedReason, session.BytesCaptured,
		session.Warnings, session.TerminalCols, session.TerminalRows,
	)
	return err
}

func (s *Store) UpdateSession(sessionID string, patch model.SessionPatch) error {
	query := `
UPDATE sessions SET
    status = COALESCE($2, status),
    ended_at = COALESCE($3, ended_at),
    duration_ms = COALESCE($4, duration_ms),
    exit_code = COALESCE($5, exit_code),
    parser_confidence = COALESCE($6, parser_confidence),
    observation_confidence = COALESCE($7, observation_confidence),
    auth_mode = COALESCE($8, auth_mode),
    blocked_reason = COALESCE($9, blocked_reason),
    bytes_captured = COALESCE($10, bytes_captured),
    warnings = COALESCE($11, warnings),
    terminal_cols = COALESCE($12, terminal_cols),
    terminal_rows = COALESCE($13, terminal_rows)
WHERE session_id = $1
`
	var status *string
	if patch.Status != nil {
		value := string(*patch.Status)
		status = &value
	}
	_, err := s.pool.Exec(context.Background(), query,
		sessionID, status, patch.EndedAt, patch.DurationMs, patch.ExitCode,
		patch.ParserConfidence, patch.ObservationConfidence, patch.AuthMode, patch.BlockedReason,
		patch.BytesCaptured, patch.Warnings, patch.TerminalCols, patch.TerminalRows,
	)
	return err
}

func (s *Store) AppendTurn(turn model.Turn) error {
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO turns (session_id, turn_index, role, content, created_at) VALUES ($1,$2,$3,$4,$5)`,
		turn.SessionID, turn.Index, turn.Role, turn.Content, turn.CreatedAt,
	)
	return err
}

func (s *Store) PeekTurns(sessionID string, lastN int) ([]model.Turn, error) {
	if lastN <= 0 {
		lastN = 10
	}
	rows, err := s.pool.Query(context.Background(),
		`SELECT session_id, turn_index, role, content, created_at
		 FROM turns WHERE session_id=$1 ORDER BY turn_index DESC LIMIT $2`,
		sessionID, lastN,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	turns := make([]model.Turn, 0, lastN)
	for rows.Next() {
		var turn model.Turn
		if err := rows.Scan(&turn.SessionID, &turn.Index, &turn.Role, &turn.Content, &turn.CreatedAt); err != nil {
			return nil, err
		}
		turns = append(turns, turn)
	}
	for i, j := 0, len(turns)-1; i < j; i, j = i+1, j-1 {
		turns[i], turns[j] = turns[j], turns[i]
	}
	return turns, rows.Err()
}

func (s *Store) AppendStreamEvent(event model.StoredStreamEvent) error {
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO stream_events (session_id, timestamp, kind, chunk_raw, chunk_text, chunk_encoding)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		event.SessionID, event.Timestamp, string(event.Kind), event.Data, base64.StdEncoding.EncodeToString(event.Data), "raw",
	)
	return err
}

func (s *Store) Close() error {
	s.pool.Close()
	return nil
}

func _timePtr(t time.Time) *time.Time { return &t }
