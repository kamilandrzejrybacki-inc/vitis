package model

import "time"

type Session struct {
	ID                    string     `json:"session_id"`
	Provider              string     `json:"provider"`
	Status                RunStatus  `json:"status"`
	StartedAt             time.Time  `json:"started_at"`
	EndedAt               *time.Time `json:"ended_at,omitempty"`
	DurationMs            *int64     `json:"duration_ms,omitempty"`
	ExitCode              *int       `json:"exit_code,omitempty"`
	ParserConfidence      *float64   `json:"parser_confidence,omitempty"`
	ObservationConfidence *float64   `json:"observation_confidence,omitempty"`
	AuthMode              string     `json:"auth_mode"`
	BlockedReason         *string    `json:"blocked_reason,omitempty"`
	BytesCaptured         *int64     `json:"bytes_captured,omitempty"`
	Warnings              []string   `json:"warnings,omitempty"`
	TerminalCols          *int       `json:"terminal_cols,omitempty"`
	TerminalRows          *int       `json:"terminal_rows,omitempty"`
}

type SessionPatch struct {
	Status                *RunStatus `json:"status,omitempty"`
	EndedAt               *time.Time `json:"ended_at,omitempty"`
	DurationMs            *int64     `json:"duration_ms,omitempty"`
	ExitCode              *int       `json:"exit_code,omitempty"`
	ParserConfidence      *float64   `json:"parser_confidence,omitempty"`
	ObservationConfidence *float64   `json:"observation_confidence,omitempty"`
	AuthMode              *string    `json:"auth_mode,omitempty"`
	BlockedReason         *string    `json:"blocked_reason,omitempty"`
	BytesCaptured         *int64     `json:"bytes_captured,omitempty"`
	Warnings              []string   `json:"warnings,omitempty"`
	TerminalCols          *int       `json:"terminal_cols,omitempty"`
	TerminalRows          *int       `json:"terminal_rows,omitempty"`
}
