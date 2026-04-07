package terminator

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

const defaultSentinel = "<<END>>"

// Compile-time assertion that Sentinel implements Terminator.
var _ Terminator = (*Sentinel)(nil)

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
			if !containsOnOwnLine(turn.Response, s.token) {
				continue
			}
			verdict := &model.Verdict{
				ConversationID: conversationID,
				Decision:       model.DecisionTerminate,
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
				Timestamp:      time.Now().UTC(),
			})
		}
	}
}

// containsOnOwnLine reports whether token appears on its own line in s.
// A "line" is a sequence bounded by \n characters or the start/end of the
// string. Leading and trailing whitespace (spaces, tabs, carriage returns)
// on the line are ignored, but the token must occupy the entire line.
func containsOnOwnLine(s, token string) bool {
	if token == "" {
		return false
	}
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimRight(line, "\r\t ") == token {
			return true
		}
	}
	return false
}

// StripSentinel removes the first line-anchored occurrence of token from
// response and returns the result. Trailing whitespace introduced by the
// strip is trimmed. Used by the broker to clean a peer's response before
// constructing the next envelope so the sentinel never leaks across.
func StripSentinel(response, token string) string {
	if token == "" {
		token = defaultSentinel
	}
	lines := strings.Split(response, "\n")
	var out []string
	for _, line := range lines {
		if strings.TrimRight(line, "\r\t ") == token {
			// Stop here; strip the sentinel and everything after.
			break
		}
		out = append(out, line)
	}
	if len(out) == len(lines) {
		// Token not found on its own line; return unchanged.
		return response
	}
	return strings.TrimRight(strings.Join(out, "\n"), " \t\r\n")
}
