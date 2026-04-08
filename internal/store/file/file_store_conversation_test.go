package file

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func newTestConvStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir, false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCreateAndUpdateConversation(t *testing.T) {
	s := newTestConvStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	conv := model.Conversation{
		ID:        "conv-test",
		CreatedAt: now,
		Status:    model.ConvRunning,
		MaxTurns:  10,
		Opener:    model.PeerSlotA,
		PeerA:     model.PeerSpec{URI: "provider:claude-code"},
		PeerB:     model.PeerSpec{URI: "provider:codex"},
	}
	require.NoError(t, s.CreateConversation(ctx, conv))

	end := now.Add(time.Minute)
	status := model.ConvCompletedSentinel
	turns := 5
	require.NoError(t, s.UpdateConversation(ctx, "conv-test", model.ConversationPatch{
		Status:        &status,
		EndedAt:       &end,
		TurnsConsumed: &turns,
	}))
}

func TestAppendAndPeekConversationTurns(t *testing.T) {
	s := newTestConvStore(t)
	ctx := context.Background()

	conv := model.Conversation{ID: "conv-x", Status: model.ConvRunning}
	require.NoError(t, s.CreateConversation(ctx, conv))

	for i := 1; i <= 5; i++ {
		require.NoError(t, s.AppendConversationTurn(ctx, model.ConversationTurn{
			ConversationID: "conv-x",
			Index:          i,
			From:           model.PeerSlotA,
			Envelope:       "env",
			Response:       "resp",
			MarkerToken:    "TURN_END_x",
			StartedAt:      time.Now().UTC(),
			EndedAt:        time.Now().UTC(),
		}))
	}

	all, err := s.PeekConversationTurns(ctx, "conv-x", 0)
	require.NoError(t, err)
	require.Len(t, all, 5)
	for i, turn := range all {
		require.Equal(t, i+1, turn.Index)
	}

	last2, err := s.PeekConversationTurns(ctx, "conv-x", 2)
	require.NoError(t, err)
	require.Len(t, last2, 2)
	require.Equal(t, 4, last2[0].Index)
	require.Equal(t, 5, last2[1].Index)
}

func TestPeekUnknownConversation(t *testing.T) {
	s := newTestConvStore(t)
	turns, err := s.PeekConversationTurns(context.Background(), "nope", 10)
	require.NoError(t, err)
	require.Empty(t, turns)
}
