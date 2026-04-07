package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/clank/internal/conversation"
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

// E2E test (real subprocesses) lives in converse_e2e_test.go.

// helper for shape assertion of FinalResult JSON shape
func decodeFinalResult(t *testing.T, raw string) conversation.FinalResult {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	var res conversation.FinalResult
	require.NoError(t, dec.Decode(&res))
	return res
}
