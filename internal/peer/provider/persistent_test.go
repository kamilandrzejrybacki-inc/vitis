package provider

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

// fakePTY simulates terminal.PseudoTerminalProcess for unit testing
// PersistentProcess without spawning real subprocesses.
type fakePTY struct {
	mu       sync.Mutex
	written  []byte
	outputCh chan model.StreamEvent
	doneCh   chan model.ExitResult
	closed   bool
}

func newFakePTY() *fakePTY {
	return &fakePTY{
		outputCh: make(chan model.StreamEvent, 64),
		doneCh:   make(chan model.ExitResult, 1),
	}
}

func (f *fakePTY) Write(data []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.written = append(f.written, data...)
	return len(data), nil
}

func (f *fakePTY) Output() <-chan model.StreamEvent { return f.outputCh }
func (f *fakePTY) Done() <-chan model.ExitResult    { return f.doneCh }
func (f *fakePTY) Terminate(_ int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		f.closed = true
		close(f.outputCh)
		f.doneCh <- model.ExitResult{Code: 0}
		close(f.doneCh)
	}
	return nil
}

// emit pushes a chunk onto the output channel.
func (f *fakePTY) emit(s string) {
	f.outputCh <- model.StreamEvent{Timestamp: time.Now(), Kind: model.StreamEventOutput, Data: []byte(s)}
}

func TestConverseTurnReturnsContentBeforeMarker(t *testing.T) {
	pty := newFakePTY()
	pp := NewPersistentProcess(pty)
	defer pp.Close(0)

	go func() {
		time.Sleep(10 * time.Millisecond)
		pty.emit("hello world\nTURN_END_aaaaaaaaaaaa\n")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := pp.ConverseTurn(ctx, []byte("envelope-1"), "TURN_END_aaaaaaaaaaaa", 500*time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, "hello world", strings.TrimSpace(string(resp)))
}

func TestConverseTurnAcrossMultipleEmits(t *testing.T) {
	pty := newFakePTY()
	pp := NewPersistentProcess(pty)
	defer pp.Close(0)

	go func() {
		pty.emit("first chunk ")
		time.Sleep(5 * time.Millisecond)
		pty.emit("second chunk\n")
		time.Sleep(5 * time.Millisecond)
		pty.emit("TURN_END_bbbbbbbbbbbb\n")
	}()
	ctx := context.Background()
	resp, err := pp.ConverseTurn(ctx, []byte("env"), "TURN_END_bbbbbbbbbbbb", 500*time.Millisecond)
	require.NoError(t, err)
	require.Contains(t, string(resp), "first chunk second chunk")
}

func TestConverseTurnSequentialTurnsAdvanceOffset(t *testing.T) {
	pty := newFakePTY()
	pp := NewPersistentProcess(pty)
	defer pp.Close(0)

	// Pre-load both turns into the channel; PersistentProcess must NOT bleed
	// turn 1 content into turn 2's response.
	go func() {
		pty.emit("turn1 reply\nTURN_END_111111111111\n")
		time.Sleep(5 * time.Millisecond)
		pty.emit("turn2 reply\nTURN_END_222222222222\n")
	}()
	ctx := context.Background()
	r1, err := pp.ConverseTurn(ctx, []byte("e1"), "TURN_END_111111111111", time.Second)
	require.NoError(t, err)
	require.Contains(t, string(r1), "turn1 reply")
	require.NotContains(t, string(r1), "turn2")

	r2, err := pp.ConverseTurn(ctx, []byte("e2"), "TURN_END_222222222222", time.Second)
	require.NoError(t, err)
	require.Contains(t, string(r2), "turn2 reply")
	require.NotContains(t, string(r2), "turn1")
}

func TestConverseTurnTimeout(t *testing.T) {
	pty := newFakePTY()
	pp := NewPersistentProcess(pty)
	defer pp.Close(0)
	ctx := context.Background()
	_, err := pp.ConverseTurn(ctx, []byte("e"), "TURN_END_neverappears", 50*time.Millisecond)
	require.Error(t, err)
	require.Contains(t, err.Error(), "timeout")
}

func TestConverseTurnContextCancel(t *testing.T) {
	pty := newFakePTY()
	pp := NewPersistentProcess(pty)
	defer pp.Close(0)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	_, err := pp.ConverseTurn(ctx, []byte("e"), "TURN_END_xxxxxxxxxxxx", time.Second)
	require.Error(t, err)
}

func TestConverseTurnProcessExits(t *testing.T) {
	pty := newFakePTY()
	pp := NewPersistentProcess(pty)
	defer pp.Close(0)
	go func() { time.Sleep(20 * time.Millisecond); _ = pty.Terminate(0) }()
	_, err := pp.ConverseTurn(context.Background(), []byte("e"), "TURN_END_xxxxxxxxxxxx", time.Second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exited")
}

func TestEnvelopeIsWrittenToPTY(t *testing.T) {
	pty := newFakePTY()
	pp := NewPersistentProcess(pty)
	defer pp.Close(0)
	go func() { time.Sleep(5 * time.Millisecond); pty.emit("ok\nTURN_END_zzzzzzzzzzzz\n") }()
	_, err := pp.ConverseTurn(context.Background(), []byte("hello envelope"), "TURN_END_zzzzzzzzzzzz", time.Second)
	require.NoError(t, err)
	pty.mu.Lock()
	written := string(pty.written)
	pty.mu.Unlock()
	require.Equal(t, "hello envelope", written)
}

// P3 regression: the marker token must be matched line-anchored, NOT as a
// substring. If the PTY echoes the envelope back, the envelope text contains
// the literal marker mid-line (in the "output the token <MARKER> on its own
// line." instruction). A naive substring search would find the marker inside
// the echo, return the echoed bytes as the "response", and drop the actual
// peer reply. This was the root cause of an intermittent E2E flake under -race.
func TestConverseTurnIgnoresMidLineMarkerInEchoedEnvelope(t *testing.T) {
	pty := newFakePTY()
	pp := NewPersistentProcess(pty)
	defer pp.Close(0)

	marker := "TURN_END_aaaaaaaaaaaa"
	go func() {
		// Simulate PTY echo of the envelope (which includes the marker mid-line)
		// followed by the actual peer reply with the marker on its own line.
		time.Sleep(5 * time.Millisecond)
		pty.emit("envelope line 1\n")
		pty.emit("When you finish your reply, output the token " + marker + " on its own line.\n")
		pty.emit("turn 1: actual reply from peer\n")
		pty.emit(marker + "\n")
	}()
	resp, err := pp.ConverseTurn(context.Background(), []byte("env"), marker, time.Second)
	require.NoError(t, err)
	respStr := string(resp)
	// The response must contain the actual peer reply, NOT just the echoed envelope.
	require.Contains(t, respStr, "actual reply from peer", "response should include the real peer reply, got: %q", respStr)
}

func TestFindLineAnchoredMarkerEdgeCases(t *testing.T) {
	marker := []byte("TURN_END_xxx")
	cases := []struct {
		name string
		buf  string
		want int // -1 means not found
	}{
		{"on its own line", "before\nTURN_END_xxx\nafter", 7},
		{"only line", "TURN_END_xxx", 0},
		{"with trailing newline", "TURN_END_xxx\n", 0},
		{"trailing whitespace", "TURN_END_xxx   \n", 0},
		{"leading whitespace", "  TURN_END_xxx\n", 2},
		{"both", "  TURN_END_xxx  \n", 2},
		{"mid-line", "output the token TURN_END_xxx now", -1},
		{"both mid-line and on own line", "output TURN_END_xxx mid\nbefore\nTURN_END_xxx\nafter", 31},
		{"absent", "no marker here", -1},
		{"empty buffer", "", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findLineAnchoredMarker([]byte(tc.buf), marker)
			require.Equal(t, tc.want, got)
		})
	}
}

// P1-3 regression: bytes that arrive AFTER the marker in the same chunk
// must NOT leak into the next turn's response. The ConverseTurn that
// observes the marker must drop trailing bytes; the next ConverseTurn must
// not see them.
func TestConverseTurnDiscardsPostMarkerBytes(t *testing.T) {
	pty := newFakePTY()
	pp := NewPersistentProcess(pty)
	defer pp.Close(0)

	// Emit turn 1's reply, marker, and post-marker chrome in a single chunk.
	go func() {
		time.Sleep(5 * time.Millisecond)
		pty.emit("turn1 reply\nTURN_END_111111111111\nleftover chrome line\n")
	}()
	r1, err := pp.ConverseTurn(context.Background(), []byte("e1"), "TURN_END_111111111111", time.Second)
	require.NoError(t, err)
	require.Equal(t, "turn1 reply", strings.TrimSpace(string(r1)))
	require.NotContains(t, string(r1), "leftover")

	// Now emit turn 2's reply. The buffer must NOT still contain
	// "leftover chrome line" from turn 1.
	go func() {
		time.Sleep(5 * time.Millisecond)
		pty.emit("turn2 reply\nTURN_END_222222222222\n")
	}()
	r2, err := pp.ConverseTurn(context.Background(), []byte("e2"), "TURN_END_222222222222", time.Second)
	require.NoError(t, err)
	require.Equal(t, "turn2 reply", strings.TrimSpace(string(r2)))
	require.NotContains(t, string(r2), "leftover", "turn 2 must not see post-marker chrome from turn 1")
	require.NotContains(t, string(r2), "chrome", "turn 2 must not see post-marker chrome from turn 1")
}
