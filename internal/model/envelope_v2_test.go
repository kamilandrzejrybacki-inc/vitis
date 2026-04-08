package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvelopeV2JSONRoundTrip(t *testing.T) {
	env := Envelope{
		ConversationID: "c1",
		TurnIndex:      3,
		MaxTurns:       10,
		From:           PeerSlotA,
		FromID:         PeerID("alice"),
		ToID:           PeerID("bob"),
		Body:           "hello",
		MarkerToken:    "<<END_T_3>>",
	}
	data, err := json.Marshal(env)
	require.NoError(t, err)

	var decoded Envelope
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, env.FromID, decoded.FromID)
	require.Equal(t, env.ToID, decoded.ToID)
	require.Equal(t, env.From, decoded.From)
}

func TestEnvelopeV1BackwardJSONDecode(t *testing.T) {
	// A legacy v1 envelope JSON must decode cleanly with empty v2 id fields.
	legacy := `{"conversation_id":"c1","turn_index":1,"max_turns":5,"from":"a","body":"hi","marker_token":"<<END>>"}`
	var env Envelope
	require.NoError(t, json.Unmarshal([]byte(legacy), &env))
	require.Equal(t, PeerSlotA, env.From)
	require.Equal(t, PeerID(""), env.FromID)
	require.Equal(t, PeerID(""), env.ToID)
}
