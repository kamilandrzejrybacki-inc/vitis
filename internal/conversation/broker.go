package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
	"github.com/kamilandrzejrybacki-inc/clank/internal/peer"
	"github.com/kamilandrzejrybacki-inc/clank/internal/terminator"
)

// ConversationStore is the narrow store interface the broker depends on.
// It is a subset of the full store.Store; the broker takes only the
// conversation methods to keep the dependency narrow and to make
// broker tests trivial to mock.
type ConversationStore interface {
	CreateConversation(ctx context.Context, conv model.Conversation) error
	UpdateConversation(ctx context.Context, conversationID string, patch model.ConversationPatch) error
	AppendConversationTurn(ctx context.Context, turn model.ConversationTurn) error
}

// BrokerDeps bundles the dependencies needed to construct a Broker.
type BrokerDeps struct {
	Conversation model.Conversation
	PeerA        peer.PeerTransport
	PeerB        peer.PeerTransport
	Terminator   terminator.Terminator
	Bus          bus.Bus
	Store        ConversationStore
}

// Broker is the conversation state machine.
type Broker struct {
	deps BrokerDeps
}

// NewBroker constructs a Broker from its dependencies.
func NewBroker(deps BrokerDeps) *Broker {
	return &Broker{deps: deps}
}

// Run drives the conversation to completion. It returns a FinalResult with
// the conversation status and turn log. Errors are reflected in the
// conversation status (ConvError, ConvPeerCrashed, etc.); they are NOT
// returned as a Go error from Run unless something catastrophic happens
// during finalization (e.g. cannot publish to the bus at all).
func (b *Broker) Run(ctx context.Context) (FinalResult, error) {
	conv := b.deps.Conversation
	conv.Status = model.ConvRunning
	conv.CreatedAt = time.Now().UTC()

	// Best-effort store create. Failures are non-blocking warnings.
	warnings := []string{}
	if err := b.deps.Store.CreateConversation(ctx, conv); err != nil {
		warnings = append(warnings, fmt.Sprintf("store create_conversation: %v", err))
	}

	// Start both peers.
	if err := b.deps.PeerA.Start(ctx, conv.PeerA, b.deps.Bus, conv.ID, model.PeerSlotA); err != nil {
		return b.finalize(ctx, conv, nil, warnings, model.ConvError, fmt.Sprintf("peer A start: %v", err))
	}
	if err := b.deps.PeerB.Start(ctx, conv.PeerB, b.deps.Bus, conv.ID, model.PeerSlotB); err != nil {
		_ = b.deps.PeerA.Stop(ctx, time.Second)
		return b.finalize(ctx, conv, nil, warnings, model.ConvError, fmt.Sprintf("peer B start: %v", err))
	}
	defer func() {
		_ = b.deps.PeerA.Stop(ctx, time.Second)
		_ = b.deps.PeerB.Stop(ctx, time.Second)
	}()

	// Start the terminator.
	if err := b.deps.Terminator.Start(ctx, conv, b.deps.Bus); err != nil {
		warnings = append(warnings, fmt.Sprintf("terminator start: %v", err))
	}
	defer b.deps.Terminator.Stop(context.Background())

	// Subscribe to control topic.
	ctlSub, ctlCancel, err := b.deps.Bus.Subscribe(ctx, bus.TopicControl(conv.ID))
	if err != nil {
		return b.finalize(ctx, conv, nil, warnings, model.ConvError, fmt.Sprintf("control subscribe: %v", err))
	}
	defer ctlCancel()

	turns := make([]model.ConversationTurn, 0, conv.MaxTurns)
	active := conv.Opener
	if active != model.PeerSlotA && active != model.PeerSlotB {
		active = model.PeerSlotA
	}

	envelope := BuildEnvelopeTurn1(conv, active, NewMarkerToken())

	for {
		select {
		case <-ctx.Done():
			conv.TurnsConsumed = len(turns)
			return b.finalize(ctx, conv, turns, warnings, model.ConvInterrupted, "context cancelled")
		default:
		}

		turn, err := b.transportFor(active).Deliver(ctx, envelope)
		if err != nil {
			conv.TurnsConsumed = len(turns)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return b.finalize(ctx, conv, turns, warnings, model.ConvInterrupted, err.Error())
			}
			return b.finalize(ctx, conv, turns, warnings, model.ConvError, fmt.Sprintf("peer %s deliver: %v", active, err))
		}

		// Persist & publish the turn.
		if err := b.deps.Store.AppendConversationTurn(ctx, turn); err != nil {
			warnings = append(warnings, fmt.Sprintf("store append_turn: %v", err))
		}
		if err := b.publishTurn(ctx, turn); err != nil {
			warnings = append(warnings, fmt.Sprintf("publish turn: %v", err))
		}

		turns = append(turns, turn)
		conv.TurnsConsumed = len(turns)

		// Hard max-turns cap.
		if conv.TurnsConsumed >= conv.MaxTurns {
			return b.finalize(ctx, conv, turns, warnings, model.ConvMaxTurnsHit, "max turns reached")
		}

		// Check context cancellation after persist/publish before draining.
		select {
		case <-ctx.Done():
			return b.finalize(ctx, conv, turns, warnings, model.ConvInterrupted, "context cancelled")
		default:
		}

		// Drain control: try a timed drain to allow async bus subscribers
		// (terminator goroutines) time to react to the published turn.
		drained := drainControlTimed(ctlSub, 5*time.Millisecond)
		for _, ctl := range drained {
			switch ctl.Kind {
			case bus.ControlVerdict:
				if ctl.Verdict != nil && ctl.Verdict.Decision == "terminate" {
					return b.finalize(ctx, conv, turns, warnings, ctl.Verdict.Status, ctl.Verdict.Reason)
				}
			case bus.ControlPeerCrashed:
				return b.finalize(ctx, conv, turns, warnings, model.ConvPeerCrashed, fmt.Sprintf("peer %s crashed: %s", ctl.Slot, ctl.Detail))
			case bus.ControlPeerBlocked:
				return b.finalize(ctx, conv, turns, warnings, model.ConvPeerBlocked, fmt.Sprintf("peer %s blocked: %s", ctl.Slot, ctl.Detail))
			}
		}

		// Build next envelope for the other peer using the (possibly stripped) response.
		nextResponse := turn.Response
		if conv.Terminator.Kind == "sentinel" {
			nextResponse = terminator.StripSentinel(nextResponse, conv.Terminator.Sentinel)
		}
		next := active.Other()
		envelope = BuildEnvelopeTurnN(conv, conv.TurnsConsumed+1, active, nextResponse, NewMarkerToken())
		active = next
	}
}

func (b *Broker) transportFor(slot model.PeerSlot) peer.PeerTransport {
	if slot == model.PeerSlotA {
		return b.deps.PeerA
	}
	return b.deps.PeerB
}

func (b *Broker) publishTurn(ctx context.Context, turn model.ConversationTurn) error {
	payload, err := json.Marshal(turn)
	if err != nil {
		return err
	}
	return b.deps.Bus.Publish(ctx, bus.TopicTurn(turn.ConversationID), bus.BusMessage{
		ConversationID: turn.ConversationID,
		Topic:          bus.TopicTurn(turn.ConversationID),
		Kind:           bus.KindTurn,
		Payload:        payload,
		Timestamp:      time.Now().UTC(),
	})
}

func (b *Broker) finalize(ctx context.Context, conv model.Conversation, turns []model.ConversationTurn, warnings []string, status model.ConversationStatus, note string) (FinalResult, error) {
	now := time.Now().UTC()
	conv.Status = status
	conv.EndedAt = &now
	if conv.TurnsConsumed == 0 && len(turns) > 0 {
		conv.TurnsConsumed = len(turns)
	}
	if err := b.deps.Store.UpdateConversation(ctx, conv.ID, model.ConversationPatch{
		Status:        &status,
		EndedAt:       &now,
		TurnsConsumed: &conv.TurnsConsumed,
	}); err != nil {
		warnings = append(warnings, fmt.Sprintf("store update_conversation: %v", err))
	}
	return FinalResult{
		Conversation:   conv,
		Turns:          turns,
		TerminatorNote: note,
		Warnings:       warnings,
	}, nil
}

// drainControl pulls every currently buffered control message off the
// channel without blocking and returns them in arrival order.
func drainControl(ch <-chan bus.BusMessage) []bus.ControlMsg {
	var out []bus.ControlMsg
	for {
		select {
		case msg, open := <-ch:
			if !open {
				return out
			}
			var ctl bus.ControlMsg
			if err := json.Unmarshal(msg.Payload, &ctl); err == nil {
				out = append(out, ctl)
			}
		default:
			return out
		}
	}
}

// drainControlTimed waits up to deadline for the first message on ch, then
// drains any additional buffered messages without blocking. This gives async
// bus subscribers (e.g. the sentinel terminator goroutine) time to react to
// a published turn before the broker checks for verdicts.
func drainControlTimed(ch <-chan bus.BusMessage, deadline time.Duration) []bus.ControlMsg {
	timer := time.NewTimer(deadline)
	defer timer.Stop()
	var out []bus.ControlMsg
	// Wait for first message or timeout.
	select {
	case msg, open := <-ch:
		if !open {
			return out
		}
		var ctl bus.ControlMsg
		if err := json.Unmarshal(msg.Payload, &ctl); err == nil {
			out = append(out, ctl)
		}
	case <-timer.C:
		return out
	}
	// Drain any additional buffered messages without blocking.
	for {
		select {
		case msg, open := <-ch:
			if !open {
				return out
			}
			var ctl bus.ControlMsg
			if err := json.Unmarshal(msg.Payload, &ctl); err == nil {
				out = append(out, ctl)
			}
		default:
			return out
		}
	}
}
