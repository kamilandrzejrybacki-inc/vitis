// Package mock provides a scripted in-memory PeerTransport used by broker
// tests. It is intentionally not in a _test.go file so that other test
// packages (e.g. integration tests) can import it.
package mock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

// Script is the canned exchange this mock will execute. Responses are
// consumed in order; if Deliver is called more times than there are
// scripted responses, the mock returns an error.
type Script struct {
	Responses []string
	Err       error // if non-nil, Deliver returns this error on the first call
}

// PeerTransport is the scripted mock implementation.
type PeerTransport struct {
	mu              sync.Mutex
	script          Script
	delivered       int
	started         bool
	stopped         bool
	conversationID  string
	slot            model.PeerSlot
	envelopeHistory []model.Envelope
}

// New constructs a mock peer transport from a Script.
func New(script Script) *PeerTransport {
	return &PeerTransport{script: script}
}

// EnvelopeHistory returns a copy of every envelope delivered so far.
func (p *PeerTransport) EnvelopeHistory() []model.Envelope {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]model.Envelope, len(p.envelopeHistory))
	copy(out, p.envelopeHistory)
	return out
}

func (p *PeerTransport) Start(_ context.Context, _ model.PeerSpec, _ bus.Bus, conversationID string, slot model.PeerSlot) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return errors.New("mock peer: already started")
	}
	p.started = true
	p.conversationID = conversationID
	p.slot = slot
	return nil
}

func (p *PeerTransport) Deliver(_ context.Context, env model.Envelope) (model.ConversationTurn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started {
		return model.ConversationTurn{}, errors.New("mock peer: deliver before start")
	}
	if p.stopped {
		return model.ConversationTurn{}, errors.New("mock peer: deliver after stop")
	}
	if p.script.Err != nil {
		err := p.script.Err
		p.script.Err = nil
		return model.ConversationTurn{}, err
	}
	if p.delivered >= len(p.script.Responses) {
		return model.ConversationTurn{}, fmt.Errorf("mock peer: script exhausted at delivery %d", p.delivered+1)
	}
	resp := p.script.Responses[p.delivered]
	p.delivered++
	p.envelopeHistory = append(p.envelopeHistory, env)
	now := time.Now().UTC()
	return model.ConversationTurn{
		ConversationID:       p.conversationID,
		Index:                env.TurnIndex,
		From:                 p.slot,
		Envelope:             env.Body,
		Response:             resp,
		MarkerToken:          env.MarkerToken,
		StartedAt:            now,
		EndedAt:              now,
		CompletionConfidence: 1.0,
		ParserConfidence:     1.0,
	}, nil
}

func (p *PeerTransport) Stop(_ context.Context, _ time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
	return nil
}
