package model

import (
	"time"
)

// ConversationStatus represents the lifecycle state of an A2A conversation.
type ConversationStatus string

const (
	ConvRunning           ConversationStatus = "running"
	ConvCompletedSentinel ConversationStatus = "completed_sentinel"
	ConvCompletedJudge    ConversationStatus = "completed_judge"
	ConvMaxTurnsHit       ConversationStatus = "max_turns_hit"
	ConvPeerCrashed       ConversationStatus = "peer_crashed"
	ConvPeerBlocked       ConversationStatus = "peer_blocked"
	ConvTimeout           ConversationStatus = "timeout"
	ConvInterrupted       ConversationStatus = "interrupted"
	ConvError             ConversationStatus = "error"
)

// PeerSlot identifies one of the two peers in a conversation.
type PeerSlot string

const (
	PeerSlotA PeerSlot = "a"
	PeerSlotB PeerSlot = "b"
)

// Other returns the opposite slot.
func (s PeerSlot) Other() PeerSlot {
	if s == PeerSlotA {
		return PeerSlotB
	}
	return PeerSlotA
}

// PeerSpec describes a peer at the URI level. The URI scheme determines which
// PeerTransport implementation handles it.
type PeerSpec struct {
	URI     string            `json:"uri"`
	Options map[string]string `json:"options,omitempty"`
}

// TerminatorSpec configures how a conversation decides when to end.
type TerminatorSpec struct {
	Kind     string `json:"kind"`                // "sentinel" | "judge"
	Sentinel string `json:"sentinel,omitempty"`  // sentinel mode token, default "<<END>>"
	JudgeURI string `json:"judge_uri,omitempty"` // judge mode URI: bus://<topic> or provider:<id>
}

// Conversation is the top-level entity for an A2A run.
type Conversation struct {
	ID             string             `json:"conversation_id"`
	CreatedAt      time.Time          `json:"created_at"`
	EndedAt        *time.Time         `json:"ended_at,omitempty"`
	Status         ConversationStatus `json:"status"`
	MaxTurns       int                `json:"max_turns"`
	PerTurnTimeout int64              `json:"per_turn_timeout_sec"`
	OverallTimeout int64              `json:"overall_timeout_sec"`
	Terminator     TerminatorSpec     `json:"terminator"`
	PeerA          PeerSpec           `json:"peer_a"`
	PeerB          PeerSpec           `json:"peer_b"`
	SeedA          string             `json:"seed_a"`
	SeedB          string             `json:"seed_b"`
	Opener         PeerSlot           `json:"opener"`
	TurnsConsumed  int                `json:"turns_consumed"`
	// ReplyStyle controls how peers should format their replies (normal,
	// caveman-lite, caveman-full, caveman-ultra). Empty string is treated
	// as "normal". The conversation/style.go package owns the canonical
	// value set; this field is a string here to keep model/ free of any
	// dependency on internal/conversation.
	ReplyStyle string `json:"reply_style,omitempty"`
}

// PerTurnTimeoutDuration returns PerTurnTimeout as a time.Duration.
func (c Conversation) PerTurnTimeoutDuration() time.Duration {
	return time.Duration(c.PerTurnTimeout) * time.Second
}

// OverallTimeoutDuration returns OverallTimeout as a time.Duration.
func (c Conversation) OverallTimeoutDuration() time.Duration {
	return time.Duration(c.OverallTimeout) * time.Second
}

// ConversationPatch is the partial update set for an existing conversation.
type ConversationPatch struct {
	Status        *ConversationStatus `json:"status,omitempty"`
	EndedAt       *time.Time          `json:"ended_at,omitempty"`
	TurnsConsumed *int                `json:"turns_consumed,omitempty"`
}

// ConversationTurn is one exchange in the conversation log.
type ConversationTurn struct {
	ConversationID       string    `json:"conversation_id"`
	Index                int       `json:"index"`
	From                 PeerSlot  `json:"from"`
	Envelope             string    `json:"envelope"`
	Response             string    `json:"response"`
	MarkerToken          string    `json:"marker_token"`
	StartedAt            time.Time `json:"started_at"`
	EndedAt              time.Time `json:"ended_at"`
	CompletionConfidence float64   `json:"completion_confidence"`
	ParserConfidence     float64   `json:"parser_confidence"`
	Warnings             []string  `json:"warnings,omitempty"`
}

// VerdictDecision is the typed decision value in a Verdict.
type VerdictDecision string

const (
	DecisionContinue  VerdictDecision = "continue"
	DecisionTerminate VerdictDecision = "terminate"
)

// Verdict is published by terminators to end a conversation.
type Verdict struct {
	ConversationID string             `json:"conversation_id"`
	Decision       VerdictDecision    `json:"decision"`
	Reason         string             `json:"reason"`
	Status         ConversationStatus `json:"status"`
}

// Envelope is the structured input handed to a peer for one turn.
// The Body field is the literal text the peer sees on stdin/PTY (or the
// 'body' field of a stdio frame). MarkerToken is the per-turn termination
// marker the peer is instructed to emit.
//
// FromID and ToID are the v2 (N-peer) routing fields and are populated
// alongside the legacy From slot during the migration window. Readers
// SHOULD prefer FromID/ToID when set and fall back to From for legacy
// 2-peer sessions.
type Envelope struct {
	ConversationID  string   `json:"conversation_id"`
	TurnIndex       int      `json:"turn_index"`
	MaxTurns        int      `json:"max_turns"`
	From            PeerSlot `json:"from"`
	FromID          PeerID   `json:"from_id,omitempty"`
	ToID            PeerID   `json:"to_id,omitempty"`
	Body            string   `json:"body"`
	MarkerToken     string   `json:"marker_token"`
	IncludeBriefing bool     `json:"include_briefing"`
	Briefing        string   `json:"briefing,omitempty"`
}
