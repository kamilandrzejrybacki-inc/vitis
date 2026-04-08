package provider

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func TestResolveAdapterKnownProviders(t *testing.T) {
	cases := []struct {
		uri string
	}{
		{"provider:claude-code"},
		{"provider:claudecode"},
		{"provider:codex"},
		{"provider:mock"},
	}
	for _, tc := range cases {
		spec := model.PeerSpec{URI: tc.uri}
		ad, err := resolveAdapter(spec)
		require.NoError(t, err, "URI %s should resolve", tc.uri)
		require.NotNil(t, ad, "URI %s should return non-nil adapter", tc.uri)
	}
}

func TestResolveAdapterUnknownProvider(t *testing.T) {
	spec := model.PeerSpec{URI: "provider:nonexistent"}
	_, err := resolveAdapter(spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown provider")
}

func TestResolveAdapterMissingPrefix(t *testing.T) {
	spec := model.PeerSpec{URI: "notprovider:foo"}
	_, err := resolveAdapter(spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported URI scheme")
}

func TestMockAdapterBuildSpawnSpec(t *testing.T) {
	opts := map[string]string{
		"bin": "/usr/bin/mockagent",
	}
	ad := newMockAdapter(opts)
	spec := ad.BuildSpawnSpec("/tmp", map[string]string{
		"MOCK_RESPONSE":         "hello",
		"MOCK_SENTINEL_AT_TURN": "2",
	}, "/home/user", 80, 24, "")

	require.Equal(t, "/usr/bin/mockagent", spec.Command)
	require.Equal(t, "hello", spec.Env["MOCK_RESPONSE"])
	require.Equal(t, "2", spec.Env["MOCK_SENTINEL_AT_TURN"])
	require.Equal(t, "1", spec.Env["MOCK_MULTI_TURN"])
}

func TestAllowedEnvKeysFiltersDisallowed(t *testing.T) {
	// Verify that the allowlist contains the keys needed by E2E tests.
	require.True(t, allowedEnvKeys["MOCK_RESPONSE"], "MOCK_RESPONSE must be in allowlist")
	require.True(t, allowedEnvKeys["MOCK_SENTINEL_AT_TURN"], "MOCK_SENTINEL_AT_TURN must be in allowlist")
	// And that dangerous keys are absent.
	require.False(t, allowedEnvKeys["LD_PRELOAD"])
	require.False(t, allowedEnvKeys["VITIS_CLAUDE_ARGS"])
}

// P1-1 regression: codex peers in converse mode must NOT spawn `codex exec`
// (one-shot mode) and must NOT include the trailing prompt argument.
func TestBuildPersistentSpawnSpecCodexNoExecOrPrompt(t *testing.T) {
	spec := model.PeerSpec{URI: "provider:codex"}
	got, err := buildPersistentSpawnSpec(spec)
	require.NoError(t, err)
	require.False(t, got.PromptInArgs, "PromptInArgs must be false in converse mode")
	for _, arg := range got.Args {
		require.NotEqual(t, "exec", arg, "codex converse spec must not include the `exec` subcommand: %v", got.Args)
		require.NotEqual(t, "", arg, "codex converse spec must not include an empty (prompt) argument: %v", got.Args)
	}
}

// P1-1 regression: claude-code peers in converse mode must not include the
// `--print` (one-shot) flag.
func TestBuildPersistentSpawnSpecClaudeCodeInteractive(t *testing.T) {
	spec := model.PeerSpec{URI: "provider:claude-code"}
	got, err := buildPersistentSpawnSpec(spec)
	require.NoError(t, err)
	require.False(t, got.PromptInArgs, "PromptInArgs must be false in converse mode")
	for _, arg := range got.Args {
		require.NotEqual(t, "--print", arg, "claude-code converse spec must not include --print: %v", got.Args)
		require.NotEqual(t, "-p", arg, "claude-code converse spec must not include -p: %v", got.Args)
	}
}

// P2-1 regression: per-peer model and reasoning-effort options are forwarded
// to the spawned subprocess via VITIS_MODEL and VITIS_REASONING_EFFORT env vars.
func TestBuildPersistentSpawnSpecForwardsModelAndReasoningEffort(t *testing.T) {
	t.Run("codex with both options", func(t *testing.T) {
		spec := model.PeerSpec{
			URI: "provider:codex",
			Options: map[string]string{
				"model":            "gpt-5",
				"reasoning-effort": "high",
			},
		}
		got, err := buildPersistentSpawnSpec(spec)
		require.NoError(t, err)
		require.Equal(t, "gpt-5", got.Env["VITIS_MODEL"])
		require.Equal(t, "high", got.Env["VITIS_REASONING_EFFORT"])
		// codex spec also surfaces them as CLI flags.
		require.Contains(t, got.Args, "--model")
		require.Contains(t, got.Args, "gpt-5")
		require.Contains(t, got.Args, "--reasoning-effort")
		require.Contains(t, got.Args, "high")
	})

	t.Run("claude-code with model only", func(t *testing.T) {
		spec := model.PeerSpec{
			URI: "provider:claude-code",
			Options: map[string]string{
				"model": "claude-sonnet-4-6",
			},
		}
		got, err := buildPersistentSpawnSpec(spec)
		require.NoError(t, err)
		require.Equal(t, "claude-sonnet-4-6", got.Env["VITIS_MODEL"])
		// claudecode adapter reads VITIS_MODEL and emits --model.
		require.Contains(t, got.Args, "--model")
		require.Contains(t, got.Args, "claude-sonnet-4-6")
	})
}
