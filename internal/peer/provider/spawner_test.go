package provider

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
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
		"MOCK_RESPONSE":        "hello",
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
	require.False(t, allowedEnvKeys["CLANK_CLAUDE_ARGS"])
}
