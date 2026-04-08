package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus"
	"github.com/kamilandrzejrybacki-inc/clank/internal/bus/inproc"
	"github.com/kamilandrzejrybacki-inc/clank/internal/conversation"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

func TestConverseRequiresPeers(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{"--seed", "hi"}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "peer-a")
}

func TestConverseRequiresSeed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer-a", "provider:mock",
		"--peer-b", "provider:mock",
	}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "seed")
}

func TestConverseRejectsAsymmetricSeedWithSingleSeed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer-a", "provider:mock",
		"--peer-b", "provider:mock",
		"--seed", "x",
		"--seed-a", "y",
	}, &stdout, &stderr)
	require.Equal(t, 2, code)
}

func TestConverseRejectsUnsupportedTerminator(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer-a", "provider:mock",
		"--peer-b", "provider:mock",
		"--seed", "x",
		"--terminator", "judge",
	}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "judge")
}

func TestConverseEnforcesMaxTurnsBounds(t *testing.T) {
	for _, mt := range []string{"0", "501"} {
		var stdout, stderr bytes.Buffer
		code := ConverseCommand(context.Background(), []string{
			"--peer-a", "provider:mock",
			"--peer-b", "provider:mock",
			"--seed", "x",
			"--max-turns", mt,
		}, &stdout, &stderr)
		require.Equal(t, 2, code, "max-turns=%s should be rejected", mt)
	}
}

func TestConverseRejectsInvalidStyle(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer-a", "provider:mock",
		"--peer-b", "provider:mock",
		"--seed", "x",
		"--style", "shouty",
	}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "style")
}

func TestConverseAcceptsValidStyles(t *testing.T) {
	// Use --max-turns 0 (rejected) so the command exits at validation
	// BEFORE spawning anything. We only care that the --style flag
	// itself is accepted, not that the conversation runs.
	for _, style := range []string{"normal", "caveman-lite", "caveman-full", "caveman-ultra"} {
		t.Run(style, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := ConverseCommand(context.Background(), []string{
				"--peer-a", "provider:mock",
				"--peer-b", "provider:mock",
				"--seed", "x",
				"--max-turns", "0",
				"--style", style,
			}, &stdout, &stderr)
			require.Equal(t, 2, code)
			// Rejection must be on max-turns, not on style.
			require.Contains(t, stderr.String(), "max-turns")
			require.NotContains(t, stderr.String(), "--style")
		})
	}
}

// E2E test (real subprocesses) lives in converse_e2e_test.go.

// helper for shape assertion of FinalResult JSON shape
func decodeFinalResult(t *testing.T, raw string) conversation.FinalResult {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	var res conversation.FinalResult
	require.NoError(t, dec.Decode(&res))
	return res
}

// P2-3 regression: streamTurnsTo must drain every published turn before
// exiting. The contract is: caller closes the bus, then waits on the
// goroutine; the goroutine must read the closed-channel sentinel after
// emitting every queued message, NOT exit early on ctx.Done.
func TestStreamTurnsToDrainsAllPublishedTurns(t *testing.T) {
	b := inproc.New()
	conversationID := "conv-stream-test"
	var out bytes.Buffer

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Subscribe BEFORE publishing so we don't lose any turns to the
	// no-buffered-subscribers branch.
	subReady := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		// We can't easily synchronise the subscribe inside the helper, so
		// just signal once we begin.
		close(subReady)
		streamTurnsTo(ctx, b, conversationID, &out)
	}()
	<-subReady
	// Tiny pause to ensure the subscribe inside streamTurnsTo lands before
	// we publish.
	time.Sleep(10 * time.Millisecond)

	// Publish three turns.
	for i := 1; i <= 3; i++ {
		turn := model.ConversationTurn{
			ConversationID: conversationID,
			Index:          i,
			From:           model.PeerSlotA,
			Response:       "reply " + string(rune('0'+i)),
		}
		payload, _ := json.Marshal(turn)
		require.NoError(t, b.Publish(ctx, bus.TopicTurn(conversationID), bus.BusMessage{
			ConversationID: conversationID,
			Topic:          bus.TopicTurn(conversationID),
			Kind:           bus.KindTurn,
			Payload:        payload,
			Timestamp:      time.Now(),
		}))
	}

	// Now close the bus. The streamTurnsTo goroutine should drain the
	// remaining buffered turns AND THEN exit because the subscription
	// channel closes.
	require.NoError(t, b.Close())
	wg.Wait()

	got := out.String()
	require.Contains(t, got, `"index":1`)
	require.Contains(t, got, `"index":2`)
	require.Contains(t, got, `"index":3`)
	require.Contains(t, got, "reply 1")
	require.Contains(t, got, "reply 2")
	require.Contains(t, got, "reply 3")
}
