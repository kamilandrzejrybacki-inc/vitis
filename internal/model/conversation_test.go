package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConversationStatusValues(t *testing.T) {
	values := []ConversationStatus{
		ConvRunning,
		ConvCompletedSentinel,
		ConvCompletedJudge,
		ConvMaxTurnsHit,
		ConvPeerCrashed,
		ConvPeerBlocked,
		ConvTimeout,
		ConvInterrupted,
		ConvError,
	}
	for _, v := range values {
		require.NotEmpty(t, string(v))
	}
}

func TestPeerSlotOther(t *testing.T) {
	require.Equal(t, PeerSlotB, PeerSlotA.Other())
	require.Equal(t, PeerSlotA, PeerSlotB.Other())
}

func TestConversationJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 4, 7, 18, 0, 0, 0, time.UTC)
	end := now.Add(5 * time.Minute)
	conv := Conversation{
		ID:             "conv-test-1",
		CreatedAt:      now,
		EndedAt:        &end,
		Status:         ConvCompletedSentinel,
		MaxTurns:       50,
		PerTurnTimeout: 300,
		OverallTimeout: 3600,
		Terminator: TerminatorSpec{
			Kind:     "sentinel",
			Sentinel: "<<END>>",
		},
		PeerA:         PeerSpec{URI: "provider:claude-code", Options: map[string]string{"model": "claude-sonnet-4-6"}},
		PeerB:         PeerSpec{URI: "provider:codex", Options: map[string]string{"model": "gpt-5"}},
		SeedA:         "Discuss X",
		SeedB:         "Discuss X",
		Opener:        PeerSlotA,
		TurnsConsumed: 7,
	}
	data, err := json.Marshal(conv)
	require.NoError(t, err)

	var decoded Conversation
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, conv.ID, decoded.ID)
	require.Equal(t, conv.Status, decoded.Status)
	require.Equal(t, conv.PeerA.URI, decoded.PeerA.URI)
	require.Equal(t, conv.PeerA.Options["model"], decoded.PeerA.Options["model"])
	require.Equal(t, conv.Opener, decoded.Opener)
	require.Equal(t, conv.TurnsConsumed, decoded.TurnsConsumed)
	require.WithinDuration(t, *conv.EndedAt, *decoded.EndedAt, time.Second)
}

func TestConversationTurnJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 4, 7, 18, 0, 0, 0, time.UTC)
	turn := ConversationTurn{
		ConversationID:       "conv-test-1",
		Index:                3,
		From:                 PeerSlotA,
		Envelope:             "[conversation: ...] hello",
		Response:             "hi back",
		MarkerToken:          "TURN_END_a7f3c1",
		StartedAt:            now,
		EndedAt:              now.Add(2 * time.Second),
		CompletionConfidence: 0.99,
		ParserConfidence:     0.97,
		Warnings:             []string{"marker_missing"},
	}
	data, err := json.Marshal(turn)
	require.NoError(t, err)
	var decoded ConversationTurn
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, turn, decoded)
}
