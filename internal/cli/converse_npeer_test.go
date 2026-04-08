package cli

import (
	"testing"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/require"
)

func TestParsePeerSpecBasic(t *testing.T) {
	pd, err := parsePeerSpec("id=alice,provider=claude-code")
	require.NoError(t, err)
	require.Equal(t, model.PeerID("alice"), pd.ID)
	require.Equal(t, "claude-code", pd.Provider)
	require.Empty(t, pd.Seed)
	require.Empty(t, pd.Options)
}

func TestParsePeerSpecWithOptions(t *testing.T) {
	pd, err := parsePeerSpec("id=bob,provider=codex,model=gpt-5,reasoning-effort=high")
	require.NoError(t, err)
	require.Equal(t, "gpt-5", pd.Options["model"])
	require.Equal(t, "high", pd.Options["reasoning-effort"])
}

func TestParsePeerSpecQuotedSeed(t *testing.T) {
	pd, err := parsePeerSpec(`id=alice,provider=claude-code,seed="hello, world."`)
	require.NoError(t, err)
	require.Equal(t, "hello, world.", pd.Seed)
}

func TestParsePeerSpecQuotedSeedWithEscapes(t *testing.T) {
	pd, err := parsePeerSpec(`id=alice,provider=claude-code,seed="say \"hi\" to bob"`)
	require.NoError(t, err)
	require.Equal(t, `say "hi" to bob`, pd.Seed)
}

func TestParsePeerSpecMissingID(t *testing.T) {
	_, err := parsePeerSpec("provider=claude-code")
	require.ErrorContains(t, err, "missing required key id")
}

func TestParsePeerSpecMissingProvider(t *testing.T) {
	_, err := parsePeerSpec("id=alice")
	require.ErrorContains(t, err, "missing required key provider")
}

func TestParsePeerSpecInvalidID(t *testing.T) {
	_, err := parsePeerSpec("id=Alice,provider=claude-code")
	require.ErrorContains(t, err, "invalid id")
}

func TestParsePeerSpecUnknownKey(t *testing.T) {
	_, err := parsePeerSpec("id=alice,provider=claude-code,bogus=x")
	require.ErrorContains(t, err, "unknown key")
}

func TestParsePeerSpecUnterminatedQuote(t *testing.T) {
	_, err := parsePeerSpec(`id=alice,provider=claude-code,seed="oops`)
	require.ErrorContains(t, err, "unterminated quoted value")
}

func TestParseNPeerSpecsHappyPath(t *testing.T) {
	raw := []string{
		"id=alice,provider=claude-code",
		"id=bob,provider=codex",
		"id=carol,provider=mock",
	}
	cfg, err := parseNPeerSpecs(raw, "broadcast", "")
	require.NoError(t, err)
	require.Len(t, cfg.Peers, 3)
	require.Equal(t, model.PeerID("alice"), cfg.OpenerID, "default opener is first declared peer")
	for _, p := range cfg.Peers {
		require.Equal(t, "broadcast", p.Seed)
	}
}

func TestParseNPeerSpecsPerPeerSeedOverridesBroadcast(t *testing.T) {
	raw := []string{
		`id=alice,provider=claude-code,seed="alice-only"`,
		"id=bob,provider=codex",
	}
	cfg, err := parseNPeerSpecs(raw, "broadcast", "")
	require.NoError(t, err)
	require.Equal(t, "alice-only", cfg.Peers[0].Seed)
	require.Equal(t, "broadcast", cfg.Peers[1].Seed)
}

func TestParseNPeerSpecsTooFew(t *testing.T) {
	_, err := parseNPeerSpecs([]string{"id=alice,provider=claude-code"}, "x", "")
	require.ErrorContains(t, err, "need at least 2 peers")
}

func TestParseNPeerSpecsTooMany(t *testing.T) {
	raw := make([]string, 17)
	for i := range raw {
		raw[i] = "id=p" + string(rune('a'+i)) + ",provider=mock"
	}
	_, err := parseNPeerSpecs(raw, "x", "")
	require.ErrorContains(t, err, "too many peers")
}

func TestParseNPeerSpecsDuplicateID(t *testing.T) {
	raw := []string{
		"id=alice,provider=claude-code",
		"id=alice,provider=codex",
	}
	_, err := parseNPeerSpecs(raw, "x", "")
	require.ErrorContains(t, err, "duplicate id")
}

func TestParseNPeerSpecsMissingSeed(t *testing.T) {
	raw := []string{
		"id=alice,provider=claude-code",
		"id=bob,provider=codex",
	}
	_, err := parseNPeerSpecs(raw, "", "")
	require.ErrorContains(t, err, "missing seed")
}

func TestParseNPeerSpecsExplicitOpener(t *testing.T) {
	raw := []string{
		"id=alice,provider=claude-code",
		"id=bob,provider=codex",
	}
	cfg, err := parseNPeerSpecs(raw, "x", "bob")
	require.NoError(t, err)
	require.Equal(t, model.PeerID("bob"), cfg.OpenerID)
}

func TestParseNPeerSpecsUnknownOpener(t *testing.T) {
	raw := []string{
		"id=alice,provider=claude-code",
		"id=bob,provider=codex",
	}
	_, err := parseNPeerSpecs(raw, "x", "ghost")
	require.ErrorContains(t, err, "not declared")
}

func TestNPeerConfigToV2Conversation(t *testing.T) {
	cfg := nPeerConfig{
		Peers: []peerDecl{
			{ID: "alice", Provider: "claude-code", Seed: "go", Options: map[string]string{"model": "x"}},
			{ID: "bob", Provider: "codex", Seed: "go"},
		},
		OpenerID: "alice",
	}
	peers, seeds := cfg.toV2Conversation()
	require.Len(t, peers, 2)
	require.Equal(t, "provider:claude-code", peers[0].Spec.URI)
	require.Equal(t, "x", peers[0].Spec.Options["model"])
	require.Equal(t, "go", seeds[model.PeerID("alice")])
	require.Equal(t, "go", seeds[model.PeerID("bob")])
}
