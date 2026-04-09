package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore is a test double for store.Store.
type mockStore struct {
	sessions      []model.Session
	conversations []model.Conversation
	turns         []model.ConversationTurn
}

func (m *mockStore) ListSessions(_ context.Context, _ model.SessionFilter) ([]model.Session, int, error) {
	return m.sessions, len(m.sessions), nil
}
func (m *mockStore) ListConversations(_ context.Context, _ model.ConversationFilter) ([]model.Conversation, int, error) {
	return m.conversations, len(m.conversations), nil
}
func (m *mockStore) GetSession(_ context.Context, id string) (*model.Session, error) {
	for _, s := range m.sessions {
		if s.ID == id {
			return &s, nil
		}
	}
	return nil, nil
}
func (m *mockStore) GetConversation(_ context.Context, id string) (*model.Conversation, error) {
	for _, c := range m.conversations {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, nil
}
func (m *mockStore) PeekConversationTurns(_ context.Context, _ string, _ int) ([]model.ConversationTurn, error) {
	return m.turns, nil
}

// Stubs for remaining Store interface methods.
func (m *mockStore) CreateSession(_ context.Context, _ model.Session) error         { return nil }
func (m *mockStore) UpdateSession(_ context.Context, _ string, _ model.SessionPatch) error {
	return nil
}
func (m *mockStore) AppendTurn(_ context.Context, _ model.Turn) error { return nil }
func (m *mockStore) PeekTurns(_ context.Context, _ string, _ int) ([]model.Turn, error) {
	return nil, nil
}
func (m *mockStore) AppendStreamEvent(_ context.Context, _ model.StoredStreamEvent) error {
	return nil
}
func (m *mockStore) CreateConversation(_ context.Context, _ model.Conversation) error { return nil }
func (m *mockStore) UpdateConversation(_ context.Context, _ string, _ model.ConversationPatch) error {
	return nil
}
func (m *mockStore) AppendConversationTurn(_ context.Context, _ model.ConversationTurn) error {
	return nil
}
func (m *mockStore) Close() error { return nil }

func TestListSessions_REST(t *testing.T) {
	ms := &mockStore{
		sessions: []model.Session{
			{ID: "s1", Provider: "claude-code", Status: model.RunStatus("completed"), StartedAt: time.Now()},
		},
	}
	cfg := Config{Port: 0}
	srv, err := NewServer(cfg, ms)
	require.NoError(t, err)
	req := httptest.NewRequest("GET", "/api/v1/sessions", nil)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp ListResponse[model.Session]
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, resp.Success)
	assert.Equal(t, 1, resp.Total)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "s1", resp.Data[0].ID)
}

func TestGetSession_REST(t *testing.T) {
	ms := &mockStore{
		sessions: []model.Session{
			{ID: "s1", Provider: "claude-code", Status: model.RunStatus("completed"), StartedAt: time.Now()},
		},
	}
	cfg := Config{Port: 0}
	srv, err := NewServer(cfg, ms)
	require.NoError(t, err)
	req := httptest.NewRequest("GET", "/api/v1/sessions/s1", nil)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetSession_REST_NotFound(t *testing.T) {
	ms := &mockStore{}
	cfg := Config{Port: 0}
	srv, err := NewServer(cfg, ms)
	require.NoError(t, err)
	req := httptest.NewRequest("GET", "/api/v1/sessions/missing", nil)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestListConversations_REST(t *testing.T) {
	ms := &mockStore{
		conversations: []model.Conversation{
			{ID: "c1", Status: model.ConversationStatus("running"), CreatedAt: time.Now()},
		},
	}
	cfg := Config{Port: 0}
	srv, err := NewServer(cfg, ms)
	require.NoError(t, err)
	req := httptest.NewRequest("GET", "/api/v1/conversations", nil)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp ListResponse[model.Conversation]
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, resp.Success)
	assert.Equal(t, 1, resp.Total)
}
