package terminator

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

const defaultSentinel = "<<END>>"

// Sentinel is a Terminator that publishes a terminate verdict the first
// time it sees the configured token in a turn response.
type Sentinel struct {
	token  string
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewSentinel returns a sentinel terminator using the given token. If the
// token is empty, the default "<<END>>" is used.
func NewSentinel(token string) *Sentinel {
	if token == "" {
		token = defaultSentinel
	}
	return &Sentinel{token: token}
}

// Start subscribes to the turn topic for the conversation and watches each
// turn response for the sentinel token. When found, it publishes a
// terminate verdict on the control topic.
func (s *Sentinel) Start(ctx context.Context, conv model.Conversation, b bus.Bus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return nil
	}
	turnSub, cancelSub, err := b.Subscribe(ctx, bus.TopicTurn(conv.ID))
	if err != nil {
		return err
	}
	runCtx, cancelRun := context.WithCancel(ctx)
	s.cancel = func() {
		cancelRun()
		cancelSub()
	}
	s.done = make(chan struct{})
	go s.loop(runCtx, conv.ID, b, turnSub)
	return nil
}

// Stop releases the subscription. Safe to call multiple times.
func (s *Sentinel) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel == nil {
		return nil
	}
	s.cancel()
	s.cancel = nil
	if s.done != nil {
		<-s.done
		s.done = nil
	}
	return nil
}

func (s *Sentinel) loop(ctx context.Context, conversationID string, b bus.Bus, turnSub <-chan bus.BusMessage) {
	defer close(s.done)
	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-turnSub:
			if !open {
				return
			}
			var turn model.ConversationTurn
			if err := json.Unmarshal(msg.Payload, &turn); err != nil {
				continue
			}
			if !strings.Contains(turn.Response, s.token) {
				continue
			}
			verdict := &model.Verdict{
				ConversationID: conversationID,
				Decision:       "terminate",
				Reason:         "sentinel token observed",
				Status:         model.ConvCompletedSentinel,
			}
			ctl := bus.ControlMsg{
				ConversationID: conversationID,
				Kind:           bus.ControlVerdict,
				Verdict:        verdict,
			}
			payload, err := json.Marshal(ctl)
			if err != nil {
				continue
			}
			_ = b.Publish(ctx, bus.TopicControl(conversationID), bus.BusMessage{
				ConversationID: conversationID,
				Topic:          bus.TopicControl(conversationID),
				Kind:           bus.KindControl,
				Payload:        payload,
			})
		}
	}
}

// StripSentinel removes the first occurrence of token from response and
// returns the result. Trailing whitespace introduced by the strip is
// trimmed. Used by the broker to clean a peer's response before
// constructing the next envelope so the sentinel never leaks across.
func StripSentinel(response, token string) string {
	if token == "" {
		token = defaultSentinel
	}
	idx := strings.Index(response, token)
	if idx < 0 {
		return response
	}
	return strings.TrimRight(response[:idx], " \t\r\n")
}
