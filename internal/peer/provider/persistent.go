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

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// maxBufferBytes is the hard cap on the PersistentProcess output buffer.
// If more than this many bytes accumulate between marker matches, ConverseTurn
// returns a buffer overflow error so the broker can finalize gracefully.
const maxBufferBytes = 64 << 20 // 64 MiB

// findLineAnchoredMarker returns the byte offset of the first occurrence of
// markerBytes that appears on its own line in buf, or -1 if no such occurrence
// exists. "On its own line" means: the marker is preceded by start-of-buffer
// or a newline (with only horizontal whitespace in between), and followed by
// end-of-buffer or a newline (with only horizontal whitespace in between).
//
// This exists because the envelope body includes a "When you finish your
// reply, output the token <MARKER> on its own line." instruction, and PTYs
// in canonical mode echo input back to output. A naive substring match
// would find the marker INSIDE the echoed envelope (mid-line) and return
// the echoed bytes as the "response", silently dropping the actual peer
// reply that arrives next. Anchoring to a full line prevents the false
// positive while still matching the marker as the peer is instructed to
// emit it (on its own line).
func findLineAnchoredMarker(buf, markerBytes []byte) int {
	if len(markerBytes) == 0 {
		return -1
	}
	offset := 0
	for {
		rel := bytes.Index(buf[offset:], markerBytes)
		if rel < 0 {
			return -1
		}
		absIdx := offset + rel
		// Check that everything between the previous newline (or start of
		// buffer) and absIdx is whitespace.
		lineStart := absIdx
		for lineStart > 0 && buf[lineStart-1] != '\n' {
			lineStart--
		}
		// Check that everything between absIdx+len(markerBytes) and the next
		// newline (or end of buffer) is whitespace.
		lineEnd := absIdx + len(markerBytes)
		for lineEnd < len(buf) && buf[lineEnd] != '\n' {
			lineEnd++
		}
		if isHorizontalWhitespace(buf[lineStart:absIdx]) && isHorizontalWhitespace(buf[absIdx+len(markerBytes):lineEnd]) {
			return absIdx
		}
		// Mid-line false positive — skip past this occurrence and keep
		// searching.
		offset = absIdx + len(markerBytes)
	}
}

func isHorizontalWhitespace(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\r' {
			return false
		}
	}
	return true
}

// stripEnvelopeEcho removes the PTY echo of the envelope from the captured
// response bytes. PTYs in canonical mode echo every byte we write back to
// the master, so the buffer between cursor and marker contains:
//
//	[echo of envelope] + [actual peer reply] + [trailing whitespace before marker]
//
// The envelope always ends with the literal instruction
//
//	"output the token <MARKER> on its own line."
//
// followed by a newline. The first occurrence of "output the token <MARKER>"
// in the buffer is therefore inside the echoed envelope (the line-anchored
// matcher above already located the LATER, on-its-own-line marker emitted
// by the peer's actual reply, so anything we see mid-line here is from the
// echo). We skip past that line and return whatever follows, trimmed.
//
// If no echo is detected (test fakePTY, or a future raw-mode PTY), the
// function returns the input unchanged so existing tests still pass.
func stripEnvelopeEcho(captured []byte, marker string) []byte {
	if len(captured) == 0 {
		return append([]byte(nil), captured...)
	}
	needle := []byte("output the token " + marker)
	echoStart := bytesIndex(captured, needle)
	if echoStart < 0 {
		return append([]byte(nil), captured...)
	}
	// Walk to the end of the echoed instruction line.
	echoLineEnd := echoStart + len(needle)
	for echoLineEnd < len(captured) && captured[echoLineEnd] != '\n' {
		echoLineEnd++
	}
	if echoLineEnd < len(captured) && captured[echoLineEnd] == '\n' {
		echoLineEnd++ // consume the newline that terminates the instruction line
	}
	// Trim any further leading whitespace (the PTY may emit \r\n or extra
	// blank lines after the echo).
	tail := captured[echoLineEnd:]
	for len(tail) > 0 && (tail[0] == '\n' || tail[0] == '\r' || tail[0] == ' ' || tail[0] == '\t') {
		tail = tail[1:]
	}
	// Trim trailing whitespace too — the marker is line-anchored, so the
	// preceding line ended with a newline that we don't want in the response.
	for len(tail) > 0 && (tail[len(tail)-1] == '\n' || tail[len(tail)-1] == '\r' || tail[len(tail)-1] == ' ' || tail[len(tail)-1] == '\t') {
		tail = tail[:len(tail)-1]
	}
	return append([]byte(nil), tail...)
}

// bytesIndex is a thin wrapper around bytes.Index that returns -1 for nil
// inputs without panicking. Defined locally to avoid pulling in another
// import line just for the safe variant.
func bytesIndex(haystack, needle []byte) int {
	if len(needle) == 0 || len(haystack) < len(needle) {
		return -1
	}
	return bytes.Index(haystack, needle)
}

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
			if idx := findLineAnchoredMarker(tail, markerBytes); idx >= 0 {
				resp := stripEnvelopeEcho(tail[:idx], marker)
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
