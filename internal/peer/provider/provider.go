package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

// Spawner constructs a raw PTY process for the given peer spec. Production
// uses terminal.Runtime.Spawn under the hood; tests inject a fake spawner
// that returns a fakePTY.
type Spawner func(ctx context.Context, spec model.PeerSpec) (rawPTYProcess, error)

// Transport is the local-PTY peer transport. It implements peer.PeerTransport.
type Transport struct {
	spawner        Spawner
	perTurnTimeout time.Duration

	mu             sync.Mutex
	process        *PersistentProcess
	conversationID string
	slot           model.PeerSlot
}

// New constructs a Transport from a Spawner and per-turn timeout.
func New(spawner Spawner, perTurnTimeout time.Duration) *Transport {
	if perTurnTimeout <= 0 {
		perTurnTimeout = 5 * time.Minute
	}
	return &Transport{
		spawner:        spawner,
		perTurnTimeout: perTurnTimeout,
	}
}

// Start spawns the underlying PTY and wraps it in a PersistentProcess.
func (t *Transport) Start(ctx context.Context, spec model.PeerSpec, _ bus.Bus, conversationID string, slot model.PeerSlot) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.process != nil {
		return fmt.Errorf("provider transport: already started")
	}
	raw, err := t.spawner(ctx, spec)
	if err != nil {
		return fmt.Errorf("spawn peer %s: %w", slot, err)
	}
	t.process = NewPersistentProcess(raw)
	t.conversationID = conversationID
	t.slot = slot
	return nil
}

// Deliver writes the envelope to the persistent PTY and waits for the
// next turn's response.
func (t *Transport) Deliver(ctx context.Context, env model.Envelope) (model.ConversationTurn, error) {
	t.mu.Lock()
	pp := t.process
	conversationID := t.conversationID
	slot := t.slot
	t.mu.Unlock()
	if pp == nil {
		return model.ConversationTurn{}, fmt.Errorf("provider transport: deliver before start")
	}

	startedAt := time.Now().UTC()
	resp, err := pp.ConverseTurn(ctx, []byte(env.Body), env.MarkerToken, t.perTurnTimeout)
	if err != nil {
		return model.ConversationTurn{}, err
	}
	endedAt := time.Now().UTC()
	return model.ConversationTurn{
		ConversationID:       conversationID,
		Index:                env.TurnIndex,
		From:                 slot,
		Envelope:             env.Body,
		Response:             string(resp),
		MarkerToken:          env.MarkerToken,
		StartedAt:            startedAt,
		EndedAt:              endedAt,
		CompletionConfidence: 0.95,
		ParserConfidence:     0.95,
	}, nil
}

// Stop terminates the persistent process with the given grace period.
func (t *Transport) Stop(_ context.Context, grace time.Duration) error {
	t.mu.Lock()
	pp := t.process
	t.process = nil
	t.mu.Unlock()
	if pp == nil {
		return nil
	}
	return pp.Close(grace)
}
