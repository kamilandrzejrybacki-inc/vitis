// Package provider implements the local-PTY peer transport for A2A
// conversations. It composes around the existing single-shot
// internal/terminal runtime, adding a turn-driven API on top of the raw
// PTY process. PTY-driven A2A is intentionally minimal in this plan: the
// only turn-end detection mechanism is per-turn marker tokens injected
// into the envelope. Sidecar-based detection (for real claude-code multi-
// turn) lands in a follow-up plan.
package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

// maxBufferBytes is the hard cap on the PersistentProcess output buffer.
// If more than this many bytes accumulate between marker matches, ConverseTurn
// returns a buffer overflow error so the broker can finalize gracefully.
const maxBufferBytes = 64 << 20 // 64 MiB

// rawPTYProcess is the narrow view of terminal.PseudoTerminalProcess that
// PersistentProcess depends on. Defining it locally lets the wrapper be
// unit-tested with a fakePTY without importing internal/terminal.
type rawPTYProcess interface {
	Write(data []byte) (int, error)
	Output() <-chan model.StreamEvent
	Done() <-chan model.ExitResult
	Terminate(gracePeriodMs int) error
}

// PersistentProcess wraps a long-lived PTY process and exposes a turn-
// driven API. Multiple ConverseTurn calls share the same buffer; the
// wrapper tracks how many bytes each turn has consumed so subsequent
// turns only see new content.
type PersistentProcess struct {
	inner rawPTYProcess

	mu       sync.Mutex
	cond     *sync.Cond
	buffer   []byte
	cursor   int // bytes already returned to previous turns
	exited   bool
	exit     model.ExitResult
	pumpDone chan struct{}
	closed   bool
}

// NewPersistentProcess constructs a wrapper around the given raw PTY.
// The caller transfers ownership of the PTY to the wrapper; calling Close
// on the wrapper terminates the underlying process.
func NewPersistentProcess(inner rawPTYProcess) *PersistentProcess {
	pp := &PersistentProcess{
		inner:    inner,
		pumpDone: make(chan struct{}),
	}
	pp.cond = sync.NewCond(&pp.mu)
	go pp.pump()
	return pp
}

func (p *PersistentProcess) pump() {
	defer close(p.pumpDone)
	out := p.inner.Output()
	done := p.inner.Done()
	for {
		select {
		case ev, open := <-out:
			if !open {
				p.mu.Lock()
				p.exited = true
				p.cond.Broadcast()
				p.mu.Unlock()
				return
			}
			p.mu.Lock()
			p.buffer = append(p.buffer, ev.Data...)
			p.cond.Broadcast()
			p.mu.Unlock()
		case res, open := <-done:
			p.mu.Lock()
			p.exited = true
			if open {
				p.exit = res
			}
			p.cond.Broadcast()
			p.mu.Unlock()
			return
		}
	}
}

// ConverseTurn writes envelopeBytes to the underlying PTY, then waits
// until either the marker token appears in newly-arrived output, the
// per-turn timeout elapses, the context is cancelled, or the process
// exits. On success it returns the bytes between the cursor and the
// marker, exclusive of the marker, and advances the cursor past the
// marker for the next turn.
func (p *PersistentProcess) ConverseTurn(ctx context.Context, envelopeBytes []byte, marker string, perTurnTimeout time.Duration) ([]byte, error) {
	if marker == "" {
		return nil, errors.New("persistent process: empty marker token")
	}
	if _, err := p.inner.Write(envelopeBytes); err != nil {
		return nil, fmt.Errorf("write envelope: %w", err)
	}

	deadline := time.Now().Add(perTurnTimeout)

	// Watcher goroutine: signals the cond on context-done or deadline so
	// the cond.Wait below wakes up promptly.
	stopWatch := make(chan struct{})
	defer close(stopWatch)
	go func() {
		t := time.NewTimer(time.Until(deadline))
		defer t.Stop()
		select {
		case <-ctx.Done():
		case <-t.C:
		case <-stopWatch:
			return
		}
		p.mu.Lock()
		p.cond.Broadcast()
		p.mu.Unlock()
	}()

	markerBytes := []byte(marker)

	p.mu.Lock()
	defer p.mu.Unlock()
	for {
		// Look in everything past the cursor.
		if p.cursor < len(p.buffer) {
			tail := p.buffer[p.cursor:]
			if idx := bytes.Index(tail, markerBytes); idx >= 0 {
				resp := append([]byte(nil), tail[:idx]...)
				// P1-3: bytes that arrived AFTER the marker in the same chunk
				// are PTY echo or shell chrome from the previous turn boundary.
				// Discarding them prevents leakage into turn N+1's response.
				// Reset the buffer entirely so the next turn starts with a
				// clean slate. Any post-marker bytes are dropped (and could
				// be surfaced as a `post_marker_chatter` warning in a future
				// pass — see spec §4 failure-handling table).
				p.buffer = nil
				p.cursor = 0
				return resp, nil
			}
			// Hard cap: if the unconsumed buffer exceeds the limit, abort.
			if len(tail) > maxBufferBytes {
				return nil, fmt.Errorf("persistent process: buffer overflow (%d bytes) waiting for marker", len(tail))
			}
		}
		if p.exited {
			return nil, fmt.Errorf("process exited (code=%d) before marker observed", p.exit.Code)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("turn timeout after %s waiting for marker %s", perTurnTimeout, marker)
		}
		p.cond.Wait()
	}
}

// Close terminates the underlying process with the given grace period and
// waits for the pump goroutine to finish. If grace is zero, a default of
// 1 second is used. Safe to call multiple times.
func (p *PersistentProcess) Close(grace time.Duration) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	if grace <= 0 {
		grace = time.Second
	}
	_ = p.inner.Terminate(int(grace.Milliseconds()))
	<-p.pumpDone
	return nil
}

// drainAvailable returns whatever output is currently buffered past the
// cursor without blocking. Used by tests and for diagnostic logging.
func (p *PersistentProcess) drainAvailable() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cursor >= len(p.buffer) {
		return nil
	}
	out := append([]byte(nil), p.buffer[p.cursor:]...)
	p.cursor = len(p.buffer)
	return out
}
