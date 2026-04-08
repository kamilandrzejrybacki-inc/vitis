package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/conversation"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func buildMockAgent(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "mockagent")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/kamilandrzejrybacki-inc/vitis/internal/testutil/mockagent")
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "building mockagent")
	return bin
}

func TestConverseEndToEndSentinelTermination(t *testing.T) {
	bin := buildMockAgent(t)
	logDir := t.TempDir()
	t.Setenv("MOCK_BIN", bin)

	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	code := ConverseCommand(ctx, []string{
		"--peer-a", "provider:mock",
		"--peer-b", "provider:mock",
		"--peer-a-opt", "env_MOCK_RESPONSE=peerA-says",
		"--peer-b-opt", "env_MOCK_RESPONSE=peerB-says",
		"--seed", "kick off",
		"--max-turns", "5",
		"--per-turn-timeout", "5",
		"--terminator", "sentinel",
		"--log-path", logDir,
		"--stream-turns=false",
	}, &stdout, &stderr)
	require.Equal(t, 0, code, "stderr: %s", stderr.String())

	// Find the FinalResult JSON object — stdout begins with the
	// (suppressed) stream and ends with the indented FinalResult.
	out := strings.TrimSpace(stdout.String())
	dec := json.NewDecoder(strings.NewReader(out))
	var res conversation.FinalResult
	require.NoError(t, dec.Decode(&res))

	// We expect the conversation to hit max-turns since the mock doesn't
	// emit a sentinel by default.
	require.Equal(t, model.ConvMaxTurnsHit, res.Conversation.Status)
	require.Equal(t, 5, len(res.Turns))
	require.Equal(t, "kick off", res.Conversation.SeedA)
}

func TestConverseEndToEndCompletesViaSentinel(t *testing.T) {
	bin := buildMockAgent(t)
	logDir := t.TempDir()
	t.Setenv("MOCK_BIN", bin)
	// Tell peer B to emit <<END>> on its third reply via env var forwarding.
	// t.Setenv only affects the test process; spawned subprocesses inherit it
	// but the mock agent is spawned via PTY from the spawner, so we pass
	// the env var via --peer-b-opt env_MOCK_SENTINEL_AT_TURN=3 instead.

	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	code := ConverseCommand(ctx, []string{
		"--peer-a", "provider:mock",
		"--peer-b", "provider:mock",
		"--peer-b-opt", "env_MOCK_SENTINEL_AT_TURN=3",
		"--seed", "go",
		"--max-turns", "20",
		"--per-turn-timeout", "5",
		"--log-path", logDir,
		"--stream-turns=false",
	}, &stdout, &stderr)
	require.Equal(t, 0, code, "stderr: %s", stderr.String())

	out := strings.TrimSpace(stdout.String())
	var res conversation.FinalResult
	require.NoError(t, json.NewDecoder(strings.NewReader(out)).Decode(&res))
	require.Equal(t, model.ConvCompletedSentinel, res.Conversation.Status)
}
