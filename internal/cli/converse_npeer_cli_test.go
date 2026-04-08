package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConverseNPeerRejectsMixingFlags ensures that --peer cannot be combined
// with the legacy --peer-a/--peer-b flags. The two modes are mutually
// exclusive at the CLI surface.
func TestConverseNPeerRejectsMixingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer", "id=alice,provider=mock",
		"--peer-a", "provider:mock",
		"--peer-b", "provider:mock",
		"--seed", "hi",
	}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "cannot mix")
}

func TestConverseNPeerRejectsTooFewPeers(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer", "id=alice,provider=mock",
		"--seed", "hi",
	}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "at least 2 peers")
}

func TestConverseNPeerRejectsDuplicateID(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer", "id=alice,provider=mock",
		"--peer", "id=alice,provider=mock",
		"--seed", "hi",
	}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "duplicate")
}

func TestConverseNPeerRejectsUnknownOpener(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer", "id=alice,provider=mock",
		"--peer", "id=bob,provider=mock",
		"--seed", "hi",
		"--opener", "ghost",
	}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "not declared")
}

func TestConverseNPeerRejectsMissingSeed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer", "id=alice,provider=mock",
		"--peer", "id=bob,provider=mock",
	}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "missing seed")
}

func TestConverseNPeerAcceptsValidConfig(t *testing.T) {
	// Use --max-turns 501 so the command exits at validation BEFORE
	// trying to spawn provider transports. We only care that flag parsing
	// + N-peer config validation passes.
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer", "id=alice,provider=mock",
		"--peer", "id=bob,provider=mock",
		"--peer", "id=carol,provider=mock",
		"--seed", "go",
		"--max-turns", "501",
	}, &stdout, &stderr)
	require.Equal(t, 2, code)
	// Rejection must be on max-turns, not on the --peer config.
	require.Contains(t, stderr.String(), "max-turns")
}

func TestConverseNPeerAcceptsPerPeerSeed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer", `id=alice,provider=mock,seed="alice seed"`,
		"--peer", `id=bob,provider=mock,seed="bob seed"`,
		"--max-turns", "501",
	}, &stdout, &stderr)
	require.Equal(t, 2, code)
	// No "missing seed" error — per-peer seeds covered every peer.
	require.NotContains(t, stderr.String(), "missing seed")
	require.Contains(t, stderr.String(), "max-turns")
}
