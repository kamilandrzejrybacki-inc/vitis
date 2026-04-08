package v1compat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/require"
)

func TestDetectV1Fixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "v1_conversation.json"))
	require.NoError(t, err)
	require.True(t, Detect(data), "v1 fixture should be detected as v1")
}

func TestDetectV2Document(t *testing.T) {
	v2 := []byte(`{"conversation_id":"c1","schema_version":2,"peers":[{"id":"alice"}]}`)
	require.False(t, Detect(v2), "v2 doc should NOT be flagged as v1")
}

func TestDetectMalformedReturnsFalse(t *testing.T) {
	require.False(t, Detect([]byte("{not json")))
}

func TestUpgradeConversationFromV1Fixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "v1_conversation.json"))
	require.NoError(t, err)

	conv, err := UpgradeConversation(data)
	require.NoError(t, err)
	require.Equal(t, 2, conv.SchemaVersion)
	require.Len(t, conv.Peers, 2)
	require.Equal(t, model.PeerID("a"), conv.Peers[0].ID)
	require.Equal(t, "provider:claude-code", conv.Peers[0].Spec.URI)
	require.Equal(t, model.PeerID("b"), conv.Peers[1].ID)
	require.Equal(t, "provider:codex", conv.Peers[1].Spec.URI)
	require.Equal(t, "you go first", conv.Seeds[model.PeerID("a")])
	require.Equal(t, "respond to peer a", conv.Seeds[model.PeerID("b")])
	require.Equal(t, model.PeerID("a"), conv.OpenerID)

	// Legacy fields preserved.
	require.Equal(t, "provider:claude-code", conv.PeerA.URI)
	require.Equal(t, model.PeerSlotA, conv.Opener)
}

func TestUpgradeConversationIsNoOpForV2(t *testing.T) {
	v2 := model.Conversation{
		ID:            "c2",
		SchemaVersion: 2,
		Peers: []model.PeerParticipant{
			{ID: "alice", Spec: model.PeerSpec{URI: "provider:claude-code"}},
		},
		OpenerID: "alice",
	}
	data, err := json.Marshal(v2)
	require.NoError(t, err)
	upgraded, err := UpgradeConversation(data)
	require.NoError(t, err)
	require.Equal(t, 2, upgraded.SchemaVersion)
	require.Len(t, upgraded.Peers, 1)
	require.Equal(t, model.PeerID("alice"), upgraded.Peers[0].ID)
}

func TestUpgradeTurnsBackfillsV2Fields(t *testing.T) {
	turns := []model.ConversationTurn{
		{Index: 1, From: model.PeerSlotA, Response: "hi"},
		{Index: 2, From: model.PeerSlotB, Response: "hello back"},
		{Index: 3, From: model.PeerSlotA, Response: "let's wrap up\n<<END>>"},
	}
	out := UpgradeTurns(turns)
	require.Len(t, out, 3)

	require.Equal(t, model.PeerID("a"), out[0].FromID)
	require.Equal(t, model.PeerID("b"), out[0].ToID)
	require.Equal(t, model.TurnReasonOpener, out[0].Reason)

	require.Equal(t, model.PeerID("b"), out[1].FromID)
	require.Equal(t, model.PeerID("a"), out[1].ToID)
	require.Equal(t, model.TurnReasonAddressed, out[1].Reason)

	require.Equal(t, model.PeerID("a"), out[2].FromID)
	require.Equal(t, model.PeerID("b"), out[2].ToID)
	require.Equal(t, model.TurnReasonAddressed, out[2].Reason)

	for _, turn := range out {
		require.False(t, turn.FallbackUsed, "legacy turns are never fallback")
		require.Nil(t, turn.NextIDParsed)
	}
}

func TestUpgradeConversationDefaultsOpenerToA(t *testing.T) {
	// A v1 doc with no opener field should default to "a" after upgrade.
	v1 := []byte(`{"conversation_id":"c","status":"running","peer_a":{"uri":"x"},"peer_b":{"uri":"y"},"seed_a":"","seed_b":"","max_turns":1,"per_turn_timeout_sec":0,"overall_timeout_sec":0,"terminator":{"kind":"sentinel","sentinel":"<<END>>"},"turns_consumed":0,"created_at":"2026-01-01T00:00:00Z"}`)
	conv, err := UpgradeConversation(v1)
	require.NoError(t, err)
	require.Equal(t, model.PeerID("a"), conv.OpenerID)
}
