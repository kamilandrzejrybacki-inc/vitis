package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConversationV2Participants(t *testing.T) {
	conv := Conversation{
		ID:            "c1",
		SchemaVersion: 2,
		Peers: []PeerParticipant{
			{ID: "alice", Spec: PeerSpec{URI: "provider:claude-code"}},
			{ID: "bob", Spec: PeerSpec{URI: "provider:codex"}},
			{ID: "carol", Spec: PeerSpec{URI: "provider:claude-code"}},
		},
		Seeds: map[PeerID]string{
			"alice": "you are the optimist",
			"bob":   "you are the pessimist",
			"carol": "you are the moderator",
		},
		OpenerID: "alice",
		MaxTurns: 12,
	}
	data, err := json.Marshal(conv)
	require.NoError(t, err)

	var decoded Conversation
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, 2, decoded.SchemaVersion)
	require.Len(t, decoded.Peers, 3)
	require.Equal(t, PeerID("alice"), decoded.Peers[0].ID)
	require.Equal(t, "you are the moderator", decoded.Seeds[PeerID("carol")])
	require.Equal(t, PeerID("alice"), decoded.OpenerID)
}

func TestConversationTurnV2Fields(t *testing.T) {
	parsed := PeerID("bob")
	turn := ConversationTurn{
		ConversationID: "c1",
		Index:          4,
		From:           PeerSlotA,
		FromID:         PeerID("alice"),
		ToID:           PeerID("bob"),
		Reason:         TurnReasonAddressed,
		NextIDParsed:   &parsed,
		FallbackUsed:   false,
	}
	data, err := json.Marshal(turn)
	require.NoError(t, err)
	var decoded ConversationTurn
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, TurnReasonAddressed, decoded.Reason)
	require.NotNil(t, decoded.NextIDParsed)
	require.Equal(t, PeerID("bob"), *decoded.NextIDParsed)
	require.Equal(t, PeerID("alice"), decoded.FromID)
	require.Equal(t, PeerID("bob"), decoded.ToID)
}

func TestConversationV1ShapeStillDecodes(t *testing.T) {
	// Legacy v1 JSON must decode cleanly with empty v2 fields.
	v1 := `{"conversation_id":"c1","status":"running","peer_a":{"uri":"provider:claude-code"},"peer_b":{"uri":"provider:codex"},"seed_a":"hi","seed_b":"hi","opener":"a","max_turns":5,"per_turn_timeout_sec":300,"overall_timeout_sec":3600,"terminator":{"kind":"sentinel","sentinel":"<<END>>"},"turns_consumed":0,"created_at":"2026-01-01T00:00:00Z"}`
	var conv Conversation
	require.NoError(t, json.Unmarshal([]byte(v1), &conv))
	require.Equal(t, 0, conv.SchemaVersion)
	require.Empty(t, conv.Peers)
	require.Equal(t, PeerSlotA, conv.Opener)
	require.Equal(t, "provider:claude-code", conv.PeerA.URI)
}
