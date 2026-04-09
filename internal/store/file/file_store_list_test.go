package file

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListSessions_Empty(t *testing.T) {
	s := newTestStore(t, false)
	sessions, total, err := s.ListSessions(context.Background(), model.SessionFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, sessions)
}

func TestListSessions_ReturnsStoredSessions(t *testing.T) {
	s := newTestStore(t, false)
	ctx := context.Background()

	sess := model.Session{
		ID:        "sess-001",
		Provider:  "claude-code",
		Status:    model.RunStatus("completed"),
		StartedAt: time.Now().UTC(),
		AuthMode:  "auto",
	}
	require.NoError(t, s.CreateSession(ctx, sess))

	sessions, total, err := s.ListSessions(ctx, model.SessionFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, sessions, 1)
	assert.Equal(t, "sess-001", sessions[0].ID)
}

func TestListSessions_FilterByStatus(t *testing.T) {
	s := newTestStore(t, false)
	ctx := context.Background()

	completed := model.RunStatus("completed")
	running := model.RunStatus("running")

	require.NoError(t, s.CreateSession(ctx, model.Session{ID: "s1", Provider: "claude-code", Status: completed, StartedAt: time.Now().UTC(), AuthMode: "auto"}))
	require.NoError(t, s.CreateSession(ctx, model.Session{ID: "s2", Provider: "claude-code", Status: running, StartedAt: time.Now().UTC(), AuthMode: "auto"}))

	sessions, total, err := s.ListSessions(ctx, model.SessionFilter{Status: &completed, Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, sessions, 1)
	assert.Equal(t, "s1", sessions[0].ID)
}

func TestListSessions_Pagination(t *testing.T) {
	s := newTestStore(t, false)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		require.NoError(t, s.CreateSession(ctx, model.Session{
			ID:        fmt.Sprintf("s%d", i),
			Provider:  "claude-code",
			Status:    model.RunStatus("completed"),
			StartedAt: time.Now().UTC(),
			AuthMode:  "auto",
		}))
	}

	sessions, total, err := s.ListSessions(ctx, model.SessionFilter{Limit: 2, Offset: 1})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, sessions, 2)
}

func TestGetSession_Found(t *testing.T) {
	s := newTestStore(t, false)
	ctx := context.Background()

	sess := model.Session{ID: "sess-get", Provider: "claude-code", Status: model.RunStatus("completed"), StartedAt: time.Now().UTC(), AuthMode: "auto"}
	require.NoError(t, s.CreateSession(ctx, sess))

	got, err := s.GetSession(ctx, "sess-get")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "sess-get", got.ID)
}

func TestGetSession_NotFound(t *testing.T) {
	s := newTestStore(t, false)
	got, err := s.GetSession(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestListConversations_Empty(t *testing.T) {
	s := newTestStore(t, false)
	convs, total, err := s.ListConversations(context.Background(), model.ConversationFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, convs)
}

func TestListConversations_ReturnsStoredConversations(t *testing.T) {
	s := newTestStore(t, false)
	ctx := context.Background()

	conv := model.Conversation{
		ID:        "conv-001",
		Status:    model.ConversationStatus("running"),
		CreatedAt: time.Now().UTC(),
		MaxTurns:  10,
		PeerA:     model.PeerSpec{URI: "claude-code"},
		PeerB:     model.PeerSpec{URI: "codex"},
	}
	require.NoError(t, s.CreateConversation(ctx, conv))

	convs, total, err := s.ListConversations(ctx, model.ConversationFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, convs, 1)
	assert.Equal(t, "conv-001", convs[0].ID)
}

func TestGetConversation_Found(t *testing.T) {
	s := newTestStore(t, false)
	ctx := context.Background()

	conv := model.Conversation{ID: "conv-get", Status: model.ConversationStatus("running"), CreatedAt: time.Now().UTC(), MaxTurns: 10}
	require.NoError(t, s.CreateConversation(ctx, conv))

	got, err := s.GetConversation(ctx, "conv-get")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "conv-get", got.ID)
}

func TestGetConversation_NotFound(t *testing.T) {
	s := newTestStore(t, false)
	got, err := s.GetConversation(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}
