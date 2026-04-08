// Package bus defines the Bus interface that backs the A2A conversation
// runtime. The Broker, peer transports, terminators, and store all
// communicate exclusively through Bus implementations.
//
// Two backends ship with vitis: an in-process channel-based Bus
// (internal/bus/inproc, default) and a NATS-backed Bus
// (internal/bus/nats, opt-in via --bus nats://...). Bus is the only
// abstraction the broker depends on; swapping backends requires no
// broker code changes.
package bus

import (
	"context"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// MessageKind tags a BusMessage payload type.
type MessageKind string

const (
	KindEnvelope MessageKind = "envelope"
	KindTurn     MessageKind = "turn"
	KindControl  MessageKind = "control"
)

// BusMessage is the on-the-wire envelope for everything published to a topic.
// Payload is JSON-encoded; the concrete type depends on Kind:
//   - KindEnvelope -> model.Envelope
//   - KindTurn     -> model.ConversationTurn
//   - KindControl  -> ControlMsg
type BusMessage struct {
	ConversationID string      `json:"conversation_id"`
	Topic          string      `json:"topic"`
	Kind           MessageKind `json:"kind"`
	Payload        []byte      `json:"payload"`
	Timestamp      time.Time   `json:"timestamp"`
}

// ControlKind tags a ControlMsg.
type ControlKind string

const (
	ControlVerdict     ControlKind = "verdict"
	ControlPeerCrashed ControlKind = "peer_crashed"
	ControlPeerBlocked ControlKind = "peer_blocked"
	ControlFinalize    ControlKind = "finalize"
)

// ControlMsg is the payload for KindControl bus messages.
type ControlMsg struct {
	ConversationID string                   `json:"conversation_id"`
	Kind           ControlKind              `json:"kind"`
	Slot           model.PeerSlot           `json:"slot,omitempty"`
	Reason         string                   `json:"reason,omitempty"`
	Status         model.ConversationStatus `json:"status,omitempty"`
	Detail         string                   `json:"detail,omitempty"`
	Verdict        *model.Verdict           `json:"verdict,omitempty"`
}

// Bus is a topic-based publish/subscribe interface. Implementations MUST
// honor the topic conventions documented in the A2A design spec
// (docs/superpowers/specs/2026-04-07-vitis-a2a-conversations-design.md):
//
//	conv/<id>/peer-a/in     envelope -> peer A transport
//	conv/<id>/peer-b/in     envelope -> peer B transport
//	conv/<id>/turn          turn responses, fan-out
//	conv/<id>/control       control messages, broker-authoritative
//
// Subscribe returns a channel of incoming messages and a cancel function.
// The cancel function MUST be called to release resources when the
// subscriber is done; failing to call it leaks goroutines and channels.
type Bus interface {
	Publish(ctx context.Context, topic string, msg BusMessage) error
	Subscribe(ctx context.Context, topic string) (<-chan BusMessage, func(), error)
	Close() error
}

// TopicEnvelopeIn returns the inbox topic for the named slot.
func TopicEnvelopeIn(conversationID string, slot model.PeerSlot) string {
	return "conv/" + conversationID + "/peer-" + string(slot) + "/in"
}

// TopicTurn returns the turn fan-out topic for a conversation.
func TopicTurn(conversationID string) string {
	return "conv/" + conversationID + "/turn"
}

// TopicControl returns the control topic for a conversation.
func TopicControl(conversationID string) string {
	return "conv/" + conversationID + "/control"
}
