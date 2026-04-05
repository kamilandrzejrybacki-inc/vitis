package model

type RunRequest struct {
	Provider     string
	Prompt       string
	PromptFile   string
	TimeoutSec   int
	Cwd          string
	EnvFile      string
	LogBackend   string
	LogPath      string
	DatabaseURL  string
	PeekLast     int
	DebugRaw     bool
	TerminalCols int
	TerminalRows int
	HomeDir      string
}

type ResultMeta struct {
	DurationMs            int64    `json:"duration_ms"`
	ExitCode              *int     `json:"exit_code,omitempty"`
	ParserConfidence      float64  `json:"parser_confidence"`
	ObservationConfidence float64  `json:"observation_confidence"`
	BytesCaptured         int64    `json:"bytes_captured"`
	BlockedReason         *string  `json:"blocked_reason,omitempty"`
	Warnings              []string `json:"warnings,omitempty"`
}

type RunResult struct {
	SessionID string     `json:"session_id"`
	Provider  string     `json:"provider"`
	Status    RunStatus  `json:"status"`
	Response  string     `json:"response"`
	Peek      []Turn     `json:"peek"`
	Meta      ResultMeta `json:"meta"`
	Error     *RunError  `json:"error,omitempty"`
}
