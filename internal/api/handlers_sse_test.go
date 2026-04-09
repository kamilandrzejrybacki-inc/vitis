package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ensure bufio and strings are used (they may be needed by future assertions).
var _ = bufio.NewScanner
var _ = strings.Contains

func TestSSE_ConversationStream_ConnectsAndReceivesHeartbeat(t *testing.T) {
	ms := &mockStore{}
	cfg := Config{Port: 0}
	srv, err := NewServer(cfg, ms)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/conversations/conv-1/stream", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 500*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	body := rr.Body.String()
	assert.Contains(t, body, "data:")
	assert.Equal(t, "text/event-stream", rr.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rr.Header().Get("Cache-Control"))
}

func TestSSE_LimitsConcurrentConnections(t *testing.T) {
	ms := &mockStore{}
	cfg := Config{Port: 0}
	srv, err := NewServer(cfg, ms)
	require.NoError(t, err)

	// Max 10 concurrent SSE
	srv.sseCount.Store(10)

	req := httptest.NewRequest("GET", "/api/v1/conversations/conv-1/stream", nil)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}
