package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/conversation/policy"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/peer"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/terminator"
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

// defaultDrainWindow is the default time budget given to async bus
// subscribers (e.g. the sentinel goroutine) to react to a published turn
// before the broker checks for verdicts. 50ms is imperceptible to users
// but provides a much wider scheduling margin than the original 5ms.
const defaultDrainWindow = 50 * time.Millisecond

// BrokerDeps bundles the dependencies needed to construct a Broker.
//
// Two transport-surface modes are supported:
//
//  1. Legacy 2-peer: set PeerA and PeerB. Leave PeersByID nil. The broker
//     synthesizes peer ids "a" and "b" from the slots and runs strict
//     alternation (or addressed routing if peers emit <<NEXT>> trailers).
//
//  2. N-peer: set PeersByID with one entry per declared peer, and PeerOrder
//     listing the peer ids in declared order (used by round-robin fallback
//     and by the broker's Start/Stop iteration). PeerA/PeerB are ignored.
//
// Mixing modes (both PeersByID and PeerA/PeerB set) is a programming
// error and produces undefined behavior — NewBroker does not validate it.
type BrokerDeps struct {
	Conversation model.Conversation
	PeerA        peer.PeerTransport
	PeerB        peer.PeerTransport
	Terminator   terminator.Terminator
	Bus          bus.Bus
	Store        ConversationStore
	// DrainWindow overrides the control-channel drain timeout. Zero means
	// use the default (50ms). Tests may set this to a shorter value.
	DrainWindow time.Duration
	// Policy selects the next speaker after each non-terminal turn. Zero
	// value means use AddressedPolicy, which for the legacy 2-peer surface
	// reduces to strict alternation when replies carry no trailer.
	Policy policy.TurnPolicy
	// PeersByID maps peer id -> transport for N-peer conversations. When
	// non-nil, PeerA/PeerB are ignored.
	PeersByID map[model.PeerID]peer.PeerTransport
	// PeerOrder is the declared order of peer ids; used by round-robin
	// fallback and by the Start/Stop iteration. Required when PeersByID
	// is set; ignored otherwise.
	PeerOrder []model.PeerID
}

// peerIDs returns the declared peer order for the active transport surface.
// In legacy 2-peer mode it returns ["a","b"]; in N-peer mode it returns
// PeerOrder.
func (b *Broker) peerIDs() []model.PeerID {
	if len(b.deps.PeersByID) > 0 && len(b.deps.PeerOrder) > 0 {
		return b.deps.PeerOrder
	}
	return []model.PeerID{"a", "b"}
}

// slotFromPeerID maps a PeerID back to its legacy PeerSlot. Only "a" and
// "b" are valid under the current 2-peer transport surface; any other id
// is an invariant violation caught by policy.Next returning an unknown id
// which should have been rejected by AddressedPolicy's membership check.
func slotFromPeerID(id model.PeerID) model.PeerSlot {
	if id == "b" {
		return model.PeerSlotB
	}
	return model.PeerSlotA
}

// Broker is the conversation state machine.
type Broker struct {
	deps BrokerDeps
}

// NewBroker constructs a Broker from its dependencies.
func NewBroker(deps BrokerDeps) *Broker {
	if deps.Policy == nil {
		deps.Policy = policy.NewAddressedPolicy()
	}
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

	// Start all declared peers (legacy 2-peer or N-peer mode).
	if err := b.startAllPeers(ctx, conv); err != nil {
		return b.finalize(ctx, conv, nil, warnings, model.ConvError, err.Error())
	}
	// Use a fresh background context so Stop succeeds even when the run
	// context has already been cancelled (H3).
	defer b.stopAllPeers()

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

	// Determine the opener slot.
	// - N-peer mode: use Conversation.OpenerID; fall back to PeerOrder[0].
	// - Legacy mode: use Conversation.Opener; fall back to PeerSlotA.
	var active model.PeerSlot
	if len(b.deps.PeersByID) > 0 {
		openerID := conv.OpenerID
		if openerID == "" && len(b.deps.PeerOrder) > 0 {
			openerID = b.deps.PeerOrder[0]
		}
		active = model.PeerSlot(openerID)
	} else {
		active = conv.Opener
		if active != model.PeerSlotA && active != model.PeerSlotB {
			active = model.PeerSlotA
		}
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

		// Populate the v2 peer-id fields on the turn record. The transport
		// returns a turn whose From is already set to the slot that just
		// replied; we mirror that into FromID and let the policy decide
		// ToID below. The Reason is filled in after the policy runs
		// (opener for the first turn, addressed or fallback afterward).
		currentID := model.PeerIDFromSlot(active)
		turn.FromID = currentID
		if len(turns) == 0 {
			turn.Reason = model.TurnReasonOpener
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

		// Run the TurnPolicy NOW (before the max-turns short-circuit) so
		// every persisted turn — including the one that hits the cap —
		// has its ToID / Reason / fallback fields populated. The chosen
		// next peer is only used if we actually continue the loop.
		nextResponse := turn.Response
		if conv.Terminator.Kind == "sentinel" {
			nextResponse = terminator.StripSentinel(nextResponse, conv.Terminator.Sentinel)
		}
		decision := b.deps.Policy.Next(currentID, nextResponse, b.peerIDs())
		last := &turns[len(turns)-1]
		last.ToID = decision.Next
		if last.Reason != model.TurnReasonOpener {
			if decision.FallbackUsed {
				last.Reason = model.TurnReasonFallbackRoundRobin
			} else {
				last.Reason = model.TurnReasonAddressed
			}
		}
		last.FallbackUsed = decision.FallbackUsed
		if decision.Parsed != nil {
			parsed := *decision.Parsed
			last.NextIDParsed = &parsed
		}

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
		drainWindow := b.deps.DrainWindow
		if drainWindow <= 0 {
			drainWindow = defaultDrainWindow
		}
		drained := drainControlTimed(ctlSub, drainWindow)
		for _, ctl := range drained {
			switch ctl.Kind {
			case bus.ControlVerdict:
				if ctl.Verdict != nil && ctl.Verdict.Decision == model.DecisionTerminate {
					return b.finalize(ctx, conv, turns, warnings, ctl.Verdict.Status, ctl.Verdict.Reason)
				}
			case bus.ControlPeerCrashed:
				return b.finalize(ctx, conv, turns, warnings, model.ConvPeerCrashed, fmt.Sprintf("peer %s crashed: %s", ctl.Slot, ctl.Detail))
			case bus.ControlPeerBlocked:
				return b.finalize(ctx, conv, turns, warnings, model.ConvPeerBlocked, fmt.Sprintf("peer %s blocked: %s", ctl.Slot, ctl.Detail))
			}
		}

		// Build next envelope for the next speaker. The policy decision
		// was computed above (before the max-turns cap) so every turn
		// record is populated consistently; here we just use the chosen
		// next peer to drive the next iteration.
		//
		// In legacy 2-peer mode the chosen id is "a" or "b" and maps
		// directly to the corresponding slot. In N-peer mode the slot
		// IS the peer id (PeerSlot is a bare string), so the same
		// conversion is a no-op.
		var next model.PeerSlot
		if len(b.deps.PeersByID) > 0 {
			next = model.PeerSlot(decision.Next)
		} else {
			next = slotFromPeerID(decision.Next)
		}
		envelope = BuildEnvelopeTurnN(conv, conv.TurnsConsumed+1, active, nextResponse, NewMarkerToken())
		active = next
	}
}

func (b *Broker) transportFor(slot model.PeerSlot) peer.PeerTransport {
	// N-peer mode: look up by peer id (the slot string IS the peer id).
	if len(b.deps.PeersByID) > 0 {
		if t, ok := b.deps.PeersByID[model.PeerID(slot)]; ok {
			return t
		}
	}
	// Legacy 2-peer mode.
	if slot == model.PeerSlotA {
		return b.deps.PeerA
	}
	return b.deps.PeerB
}

// startAllPeers starts every declared peer transport. In legacy 2-peer mode
// this is just PeerA + PeerB; in N-peer mode it iterates PeerOrder and the
// PeersByID map. On any failure, already-started peers are stopped before
// returning.
func (b *Broker) startAllPeers(ctx context.Context, conv model.Conversation) error {
	if len(b.deps.PeersByID) > 0 {
		started := make([]model.PeerID, 0, len(b.deps.PeerOrder))
		for _, id := range b.deps.PeerOrder {
			t, ok := b.deps.PeersByID[id]
			if !ok {
				b.stopPeerIDs(started)
				return fmt.Errorf("peer %s declared in PeerOrder but missing from PeersByID", id)
			}
			// The peer's spec is taken from Conversation.Peers when present;
			// fall back to legacy PeerA/PeerB for the "a"/"b" ids.
			spec := b.specForPeerID(conv, id)
			if err := t.Start(ctx, spec, b.deps.Bus, conv.ID, model.PeerSlot(id)); err != nil {
				b.stopPeerIDs(started)
				return fmt.Errorf("peer %s start: %w", id, err)
			}
			started = append(started, id)
		}
		return nil
	}
	// Legacy 2-peer path.
	if err := b.deps.PeerA.Start(ctx, conv.PeerA, b.deps.Bus, conv.ID, model.PeerSlotA); err != nil {
		return fmt.Errorf("peer A start: %w", err)
	}
	if err := b.deps.PeerB.Start(ctx, conv.PeerB, b.deps.Bus, conv.ID, model.PeerSlotB); err != nil {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = b.deps.PeerA.Stop(stopCtx, time.Second)
		stopCancel()
		return fmt.Errorf("peer B start: %w", err)
	}
	return nil
}

// stopAllPeers stops every started peer transport. In legacy mode it
// stops PeerA and PeerB; in N-peer mode it iterates PeersByID.
func (b *Broker) stopAllPeers() {
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if len(b.deps.PeersByID) > 0 {
		for _, id := range b.deps.PeerOrder {
			if t, ok := b.deps.PeersByID[id]; ok {
				_ = t.Stop(stopCtx, time.Second)
			}
		}
		return
	}
	_ = b.deps.PeerA.Stop(stopCtx, time.Second)
	_ = b.deps.PeerB.Stop(stopCtx, time.Second)
}

func (b *Broker) stopPeerIDs(ids []model.PeerID) {
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	for _, id := range ids {
		if t, ok := b.deps.PeersByID[id]; ok {
			_ = t.Stop(stopCtx, time.Second)
		}
	}
}

// specForPeerID returns the PeerSpec for the named peer id. v2 conversations
// carry it in Conversation.Peers; legacy 2-peer conversations are looked up
// via the PeerA/PeerB slots.
func (b *Broker) specForPeerID(conv model.Conversation, id model.PeerID) model.PeerSpec {
	for _, p := range conv.Peers {
		if p.ID == id {
			return p.Spec
		}
	}
	if id == "a" {
		return conv.PeerA
	}
	if id == "b" {
		return conv.PeerB
	}
	return model.PeerSpec{}
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
