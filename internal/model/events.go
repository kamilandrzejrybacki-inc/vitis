package model

import "time"

type StreamEventKind string

const (
	StreamEventInput  StreamEventKind = "stdin"
	StreamEventOutput StreamEventKind = "pty"
)

type StreamEvent struct {
	Timestamp time.Time       `json:"timestamp"`
	Kind      StreamEventKind `json:"kind"`
	Data      []byte          `json:"data"`
}

type StoredStreamEvent struct {
	SessionID string          `json:"session_id"`
	Timestamp time.Time       `json:"timestamp"`
	Kind      StreamEventKind `json:"kind"`
	Data      []byte          `json:"data"`
}

type ExitResult struct {
	Code int   `json:"code"`
	Err  error `json:"-"`
}
