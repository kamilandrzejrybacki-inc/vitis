// Package inproc is the default in-process Bus backend. It is a
// channel-fanout broker with no external dependencies. It is the
// correct choice for single-machine, single-process clank converse
// runs. For distributed peers, observability, or external judges,
// use internal/bus/nats instead.
package inproc

import (
	"context"
	"errors"
	"sync"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus"
)

// Compile-time assertion that Bus implements bus.Bus.
var _ bus.Bus = (*Bus)(nil)

// Default subscriber buffer size. Tunable per-Bus via WithBufferSize.
const defaultBufferSize = 64

// Option configures a Bus at construction time.
type Option func(*Bus)

// WithBufferSize overrides the per-subscriber channel buffer size.
func WithBufferSize(n int) Option {
	return func(b *Bus) {
		if n > 0 {
			b.bufferSize = n
		}
	}
}

// Bus is the in-process Bus implementation.
type Bus struct {
	mu         sync.RWMutex
	closed     bool
	bufferSize int
	subs       map[string][]*subscription
}

type subscription struct {
	ch     chan bus.BusMessage
	closed bool
	mu     sync.Mutex
}

// New constructs an in-process Bus.
func New(opts ...Option) *Bus {
	b := &Bus{
		bufferSize: defaultBufferSize,
		subs:       make(map[string][]*subscription),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Publish fans the message out to every current subscriber on topic.
// A subscriber whose channel is full has the message dropped silently
// (a warning is the caller's responsibility — bus implementations must
// not block on slow consumers).
func (b *Bus) Publish(_ context.Context, topic string, msg bus.BusMessage) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return errors.New("inproc bus: publish on closed bus")
	}
	for _, sub := range b.subs[topic] {
		sub.deliver(msg)
	}
	return nil
}

// Subscribe registers a new subscriber on topic and returns its channel
// plus a cancel function. The cancel function unsubscribes and closes
// the channel; it is safe to call multiple times.
func (b *Bus) Subscribe(_ context.Context, topic string) (<-chan bus.BusMessage, func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, nil, errors.New("inproc bus: subscribe on closed bus")
	}
	sub := &subscription{ch: make(chan bus.BusMessage, b.bufferSize)}
	b.subs[topic] = append(b.subs[topic], sub)

	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		// Remove sub from b.subs[topic]
		list := b.subs[topic]
		for i, s := range list {
			if s == sub {
				b.subs[topic] = append(list[:i], list[i+1:]...)
				break
			}
		}
		sub.close()
	}
	return sub.ch, cancel, nil
}

// Close closes every subscription and marks the bus closed. After Close,
// Publish and Subscribe both return errors.
func (b *Bus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for _, list := range b.subs {
		for _, sub := range list {
			sub.close()
		}
	}
	b.subs = make(map[string][]*subscription)
	return nil
}

func (s *subscription) deliver(msg bus.BusMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.ch <- msg:
	default:
		// Slow consumer: drop. The publisher does not block.
	}
}

func (s *subscription) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.ch)
}
