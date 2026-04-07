// Package mock provides a scripted in-memory PeerTransport used by broker
// tests. It is intentionally not in a _test.go file so that other test
// packages (e.g. integration tests) can import it.
package mock

import (
	"context"
	"errors"
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

// Transport is the scripted mock implementation of peer.PeerTransport.
// The type is named Transport (matching provider.Transport) to avoid
// shadowing the peer.PeerTransport interface it implements.
type Transport struct {
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
func New(script Script) *Transport {
	return &Transport{script: script}
}

// EnvelopeHistory returns a copy of every envelope delivered so far.
func (p *Transport) EnvelopeHistory() []model.Envelope {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]model.Envelope, len(p.envelopeHistory))
	copy(out, p.envelopeHistory)
	return out
}

func (p *Transport) Start(_ context.Context, _ model.PeerSpec, _ bus.Bus, conversationID string, slot model.PeerSlot) error {
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

func (p *Transport) Deliver(ctx context.Context, env model.Envelope) (model.ConversationTurn, error) {
	// Check context before acquiring lock so cancellation is detected promptly.
	select {
	case <-ctx.Done():
		return model.ConversationTurn{}, ctx.Err()
	default:
	}
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return model.ConversationTurn{}, errors.New("mock peer: deliver before start")
	}
	if p.stopped {
		p.mu.Unlock()
		return model.ConversationTurn{}, errors.New("mock peer: deliver after stop")
	}
	if p.script.Err != nil {
		err := p.script.Err
		p.script.Err = nil
		p.mu.Unlock()
		return model.ConversationTurn{}, err
	}
	if p.delivered >= len(p.script.Responses) {
		p.mu.Unlock()
		// Script exhausted: block until context is cancelled so tests that
		// cancel mid-conversation can observe ConvInterrupted instead of
		// ConvError.
		<-ctx.Done()
		return model.ConversationTurn{}, ctx.Err()
	}
	defer p.mu.Unlock()
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

func (p *Transport) Stop(_ context.Context, _ time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
	return nil
}
