# Vitis Event API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an HTTP event API to Vitis (`vitis serve`) exposing REST endpoints for stored sessions/conversations and SSE streams for live conversation events.

**Architecture:** New `internal/api/` package with HTTP server, REST handlers reading from the Store interface, and SSE handlers using fsnotify to detect file changes from concurrent `vitis converse` processes. New `vitis serve` CLI subcommand wires Store into the API server.

**Tech Stack:** Go 1.24, net/http stdlib, fsnotify, encoding/json, existing Store + model packages

**Spec:** `docs/superpowers/specs/2026-04-09-vitis-event-api-and-ui-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/model/filters.go` | Create | SessionFilter, ConversationFilter types |
| `internal/store/store.go` | Modify | Add List/Get methods to Store interface |
| `internal/store/file/file_store_list.go` | Create | File store implementation of List/Get methods |
| `internal/store/file/file_store_list_test.go` | Create | Tests for List/Get methods |
| `internal/api/server.go` | Create | HTTP server setup, route registration, graceful shutdown |
| `internal/api/server_test.go` | Create | Server lifecycle tests |
| `internal/api/handlers_rest.go` | Create | REST handlers for sessions/conversations |
| `internal/api/handlers_rest_test.go` | Create | REST handler tests |
| `internal/api/handlers_sse.go` | Create | SSE handlers for live event streaming |
| `internal/api/handlers_sse_test.go` | Create | SSE handler tests |
| `internal/api/watcher.go` | Create | fsnotify file watcher for live event detection |
| `internal/api/watcher_test.go` | Create | Watcher tests |
| `internal/api/middleware.go` | Create | CORS, logging, optional API key auth |
| `internal/api/middleware_test.go` | Create | Middleware tests |
| `internal/cli/serve.go` | Create | `vitis serve` CLI subcommand |
| `internal/cli/serve_test.go` | Create | CLI flag parsing tests |

---

### Task 1: Model Filter Types

**Files:**
- Create: `internal/model/filters.go`

- [ ] **Step 1: Create filter types**

```go
// internal/model/filters.go
package model

// SessionFilter controls listing of stored sessions.
type SessionFilter struct {
	Status *RunStatus
	Limit  int
	Offset int
}

// ConversationFilter controls listing of stored conversations.
type ConversationFilter struct {
	Status *ConversationStatus
	Limit  int
	Offset int
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/kamil-rybacki/Code/vitis && go build ./internal/model/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis
git add internal/model/filters.go
git commit -m "feat(model): add SessionFilter and ConversationFilter types"
```

---

### Task 2: Extend Store Interface

**Files:**
- Modify: `internal/store/store.go:9-23`

- [ ] **Step 1: Add List/Get methods to Store interface**

Add these four methods to the existing `Store` interface in `internal/store/store.go`, after the `PeekConversationTurns` line and before `Close() error`:

```go
	// Query methods for the API layer.
	ListSessions(ctx context.Context, filter model.SessionFilter) ([]model.Session, int, error)
	ListConversations(ctx context.Context, filter model.ConversationFilter) ([]model.Conversation, int, error)
	GetSession(ctx context.Context, sessionID string) (*model.Session, error)
	GetConversation(ctx context.Context, conversationID string) (*model.Conversation, error)
```

- [ ] **Step 2: Verify it compiles (expect failures in implementations)**

Run: `cd /home/kamil-rybacki/Code/vitis && go build ./internal/store/...`
Expected: compilation errors in `file_store.go` and `postgres_store.go` — they don't implement the new methods yet. This is expected.

- [ ] **Step 3: Add stub implementations to Postgres store**

Add to `internal/store/postgres/postgres_store.go` at the end:

```go
func (s *Store) ListSessions(_ context.Context, _ model.SessionFilter) ([]model.Session, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}

func (s *Store) ListConversations(_ context.Context, _ model.ConversationFilter) ([]model.Conversation, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}

func (s *Store) GetSession(_ context.Context, _ string) (*model.Session, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *Store) GetConversation(_ context.Context, _ string) (*model.Conversation, error) {
	return nil, fmt.Errorf("not implemented")
}
```

Add `"fmt"` to the import block if not already present.

- [ ] **Step 4: Verify it compiles**

Run: `cd /home/kamil-rybacki/Code/vitis && go build ./internal/store/...`
Expected: compilation errors only for file store (postgres stubs satisfy the interface)

- [ ] **Step 5: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis
git add internal/store/store.go internal/store/postgres/postgres_store.go
git commit -m "feat(store): add ListSessions, ListConversations, GetSession, GetConversation to Store interface"
```

---

### Task 3: File Store List/Get Implementation

**Files:**
- Create: `internal/store/file/file_store_list.go`
- Create: `internal/store/file/file_store_list_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/store/file/file_store_list_test.go
package file

import (
	"context"
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListSessions_Empty(t *testing.T) {
	s := newTestStore(t)
	sessions, total, err := s.ListSessions(context.Background(), model.SessionFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, sessions)
}

func TestListSessions_ReturnsStoredSessions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	sess := model.Session{
		ID:        "sess-001",
		Provider:  "claude-code",
		Status:    model.RunStatus("completed"),
		StartedAt: time.Now(),
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
	s := newTestStore(t)
	ctx := context.Background()

	completed := model.RunStatus("completed")
	running := model.RunStatus("running")

	require.NoError(t, s.CreateSession(ctx, model.Session{ID: "s1", Provider: "claude-code", Status: completed, StartedAt: time.Now(), AuthMode: "auto"}))
	require.NoError(t, s.CreateSession(ctx, model.Session{ID: "s2", Provider: "claude-code", Status: running, StartedAt: time.Now(), AuthMode: "auto"}))

	sessions, total, err := s.ListSessions(ctx, model.SessionFilter{Status: &completed, Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, sessions, 1)
	assert.Equal(t, "s1", sessions[0].ID)
}

func TestListSessions_Pagination(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		require.NoError(t, s.CreateSession(ctx, model.Session{
			ID: fmt.Sprintf("s%d", i), Provider: "claude-code",
			Status: model.RunStatus("completed"), StartedAt: time.Now(), AuthMode: "auto",
		}))
	}

	sessions, total, err := s.ListSessions(ctx, model.SessionFilter{Limit: 2, Offset: 1})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, sessions, 2)
}

func TestGetSession_Found(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	sess := model.Session{ID: "sess-get", Provider: "claude-code", Status: model.RunStatus("completed"), StartedAt: time.Now(), AuthMode: "auto"}
	require.NoError(t, s.CreateSession(ctx, sess))

	got, err := s.GetSession(ctx, "sess-get")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "sess-get", got.ID)
}

func TestGetSession_NotFound(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetSession(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestListConversations_Empty(t *testing.T) {
	s := newTestStore(t)
	convs, total, err := s.ListConversations(context.Background(), model.ConversationFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, convs)
}

func TestListConversations_ReturnsStoredConversations(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	conv := model.Conversation{
		ID:        "conv-001",
		Status:    model.ConversationStatus("running"),
		CreatedAt: time.Now(),
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
	s := newTestStore(t)
	ctx := context.Background()

	conv := model.Conversation{ID: "conv-get", Status: model.ConversationStatus("running"), CreatedAt: time.Now(), MaxTurns: 10}
	require.NoError(t, s.CreateConversation(ctx, conv))

	got, err := s.GetConversation(ctx, "conv-get")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "conv-get", got.ID)
}

func TestGetConversation_NotFound(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetConversation(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}
```

Add `"fmt"` to the import block.

Note: `newTestStore` should already be defined in the existing test file. Check `internal/store/file/file_store_test.go` for its definition — it creates a temp directory and returns `*Store`. If it's not exported from the test package, add a helper:

```go
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir, false)
	require.NoError(t, err)
	return s
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/store/file/... -run "TestList|TestGet" -v 2>&1 | head -30`
Expected: FAIL — `ListSessions`, `ListConversations`, `GetSession`, `GetConversation` not defined on `*Store`

- [ ] **Step 3: Implement List/Get methods**

```go
// internal/store/file/file_store_list.go
package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func (s *Store) ListSessions(_ context.Context, filter model.SessionFilter) ([]model.Session, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}

	var all []model.Session
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		var sess model.Session
		if readErr := s.readJSON(filepath.Join(dir, e.Name()), &sess); readErr != nil {
			continue
		}
		if filter.Status != nil && sess.Status != *filter.Status {
			continue
		}
		all = append(all, sess)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].StartedAt.After(all[j].StartedAt)
	})

	total := len(all)
	if filter.Offset > 0 && filter.Offset < len(all) {
		all = all[filter.Offset:]
	} else if filter.Offset >= len(all) {
		all = nil
	}
	if filter.Limit > 0 && len(all) > filter.Limit {
		all = all[:filter.Limit]
	}

	return all, total, nil
}

func (s *Store) ListConversations(_ context.Context, filter model.ConversationFilter) ([]model.Conversation, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.conversationsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}

	var all []model.Conversation
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(dir, e.Name(), "conversation.json")
		var conv model.Conversation
		data, readErr := os.ReadFile(metaPath)
		if readErr != nil {
			continue
		}
		if jsonErr := json.Unmarshal(data, &conv); jsonErr != nil {
			continue
		}
		if filter.Status != nil && conv.Status != *filter.Status {
			continue
		}
		all = append(all, conv)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	total := len(all)
	if filter.Offset > 0 && filter.Offset < len(all) {
		all = all[filter.Offset:]
	} else if filter.Offset >= len(all) {
		all = nil
	}
	if filter.Limit > 0 && len(all) > filter.Limit {
		all = all[:filter.Limit]
	}

	return all, total, nil
}

func (s *Store) GetSession(_ context.Context, sessionID string) (*model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.sessionPath(sessionID)
	var sess model.Session
	if err := s.readJSON(path, &sess); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &sess, nil
}

func (s *Store) GetConversation(_ context.Context, conversationID string) (*model.Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.conversationPath(conversationID)
	var conv model.Conversation
	if err := s.readJSON(path, &conv); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &conv, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/store/file/... -run "TestList|TestGet" -v`
Expected: all PASS

- [ ] **Step 5: Run full store test suite to check for regressions**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/store/... -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis
git add internal/store/file/file_store_list.go internal/store/file/file_store_list_test.go
git commit -m "feat(store): implement ListSessions, ListConversations, GetSession, GetConversation for file store"
```

---

### Task 4: API Server Skeleton

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/server_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/api/server_test.go
package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_StartsAndStops(t *testing.T) {
	cfg := Config{Port: 0} // 0 = OS picks a free port
	srv, err := NewServer(cfg, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe(ctx) }()

	// Wait for server to be ready
	time.Sleep(50 * time.Millisecond)

	addr := srv.Addr()
	require.NotEmpty(t, addr)

	resp, err := http.Get("http://" + addr + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	cancel()
	err = <-errCh
	assert.NoError(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/api/... -run TestServer_StartsAndStops -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement server skeleton**

```go
// internal/api/server.go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/store"
)

// Config holds configuration for the API server.
type Config struct {
	Port      int
	APIKey    string
	CORSOrigin string
}

// Server is the Vitis HTTP event API server.
type Server struct {
	cfg       Config
	store     store.Store
	mux       *http.ServeMux
	listener  net.Listener
	startedAt time.Time
	sseCount  atomic.Int64
}

// HealthResponse is the /health endpoint response.
type HealthResponse struct {
	Status    string `json:"status"`
	Store     string `json:"store"`
}

// StatusResponse is the /api/v1/status endpoint response.
type StatusResponse struct {
	ActiveConversations int    `json:"active_conversations"`
	ActiveSSEStreams    int64  `json:"active_sse_streams"`
	UptimeSeconds       int64  `json:"uptime_seconds"`
	StoreBackend        string `json:"store_backend"`
	Version             string `json:"version"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error  string `json:"error"`
	Detail string `json:"detail"`
}

// NewServer creates a new API server. Pass nil store for health-only mode.
func NewServer(cfg Config, s store.Store) (*Server, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	srv := &Server{
		cfg:       cfg,
		store:     s,
		mux:       http.NewServeMux(),
		listener:  ln,
		startedAt: time.Now(),
	}
	srv.registerRoutes()
	return srv, nil
}

// Addr returns the listener address (useful when Port=0).
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// ListenAndServe runs the server until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	httpSrv := &http.Server{Handler: s.mux}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpSrv.Serve(s.listener)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/status", s.handleStatus)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	storeType := "none"
	if s.store != nil {
		storeType = "file"
	}
	writeJSON(w, http.StatusOK, HealthResponse{
		Status: "ok",
		Store:  storeType,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, StatusResponse{
		ActiveConversations: 0,
		ActiveSSEStreams:    s.sseCount.Load(),
		UptimeSeconds:       int64(time.Since(s.startedAt).Seconds()),
		StoreBackend:        "file",
		Version:             "0.5.0",
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, errCode, detail string) {
	writeJSON(w, status, ErrorResponse{Error: errCode, Detail: detail})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/api/... -run TestServer_StartsAndStops -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis
git add internal/api/server.go internal/api/server_test.go
git commit -m "feat(api): add HTTP server skeleton with /health and /api/v1/status endpoints"
```

---

### Task 5: CORS and API Key Middleware

**Files:**
- Create: `internal/api/middleware.go`
- Create: `internal/api/middleware_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/api/middleware_test.go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCORSMiddleware_AllowsLocalhost(t *testing.T) {
	handler := corsMiddleware("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "http://localhost:5173", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_RejectsNonLocalhost(t *testing.T) {
	handler := corsMiddleware("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_CustomOrigin(t *testing.T) {
	handler := corsMiddleware("https://mydomain.com", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://mydomain.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "https://mydomain.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestAPIKeyMiddleware_NoKeyConfigured(t *testing.T) {
	handler := apiKeyMiddleware("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
	handler := apiKeyMiddleware("secret123", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPIKeyMiddleware_InvalidKey(t *testing.T) {
	handler := apiKeyMiddleware("secret123", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAPIKeyMiddleware_MissingHeader(t *testing.T) {
	handler := apiKeyMiddleware("secret123", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/api/... -run "TestCORS|TestAPIKey" -v`
Expected: FAIL — `corsMiddleware`, `apiKeyMiddleware` not defined

- [ ] **Step 3: Implement middleware**

```go
// internal/api/middleware.go
package api

import (
	"net/http"
	"strings"
)

func corsMiddleware(customOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		allowed := false
		if customOrigin != "" && origin == customOrigin {
			allowed = true
		} else if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
			allowed = true
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func apiKeyMiddleware(key string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid Authorization header")
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != key {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/api/... -run "TestCORS|TestAPIKey" -v`
Expected: all PASS

- [ ] **Step 5: Wire middleware into server routes**

Update `registerRoutes()` in `internal/api/server.go`:

```go
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)

	apiHandler := http.NewServeMux()
	apiHandler.HandleFunc("GET /api/v1/status", s.handleStatus)

	var handler http.Handler = apiHandler
	handler = apiKeyMiddleware(s.cfg.APIKey, handler)
	handler = corsMiddleware(s.cfg.CORSOrigin, handler)
	s.mux.Handle("/api/", handler)
}
```

- [ ] **Step 6: Run full test suite**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/api/... -v`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis
git add internal/api/middleware.go internal/api/middleware_test.go internal/api/server.go
git commit -m "feat(api): add CORS and API key middleware"
```

---

### Task 6: REST Handlers for Sessions and Conversations

**Files:**
- Create: `internal/api/handlers_rest.go`
- Create: `internal/api/handlers_rest_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/api/handlers_rest_test.go
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

// mockStore implements store.Store for testing REST handlers.
type mockStore struct {
	sessions      []model.Session
	conversations []model.Conversation
	convTurns     map[string][]model.ConversationTurn
	sessTurns     map[string][]model.Turn
}

func (m *mockStore) ListSessions(_ context.Context, f model.SessionFilter) ([]model.Session, int, error) {
	var filtered []model.Session
	for _, s := range m.sessions {
		if f.Status != nil && s.Status != *f.Status {
			continue
		}
		filtered = append(filtered, s)
	}
	total := len(filtered)
	if f.Offset > 0 && f.Offset < len(filtered) {
		filtered = filtered[f.Offset:]
	}
	if f.Limit > 0 && len(filtered) > f.Limit {
		filtered = filtered[:f.Limit]
	}
	return filtered, total, nil
}

func (m *mockStore) ListConversations(_ context.Context, f model.ConversationFilter) ([]model.Conversation, int, error) {
	var filtered []model.Conversation
	for _, c := range m.conversations {
		if f.Status != nil && c.Status != *f.Status {
			continue
		}
		filtered = append(filtered, c)
	}
	total := len(filtered)
	if f.Offset > 0 && f.Offset < len(filtered) {
		filtered = filtered[f.Offset:]
	}
	if f.Limit > 0 && len(filtered) > f.Limit {
		filtered = filtered[:f.Limit]
	}
	return filtered, total, nil
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

func (m *mockStore) PeekTurns(_ context.Context, sessionID string, lastN int) ([]model.Turn, error) {
	turns := m.sessTurns[sessionID]
	if lastN > 0 && len(turns) > lastN {
		turns = turns[len(turns)-lastN:]
	}
	return turns, nil
}

func (m *mockStore) PeekConversationTurns(_ context.Context, convID string, lastN int) ([]model.ConversationTurn, error) {
	turns := m.convTurns[convID]
	if lastN > 0 && len(turns) > lastN {
		turns = turns[len(turns)-lastN:]
	}
	return turns, nil
}

// Unused store methods — satisfy interface.
func (m *mockStore) CreateSession(context.Context, model.Session) error                          { return nil }
func (m *mockStore) UpdateSession(context.Context, string, model.SessionPatch) error             { return nil }
func (m *mockStore) AppendTurn(context.Context, model.Turn) error                                { return nil }
func (m *mockStore) AppendStreamEvent(context.Context, model.StoredStreamEvent) error            { return nil }
func (m *mockStore) CreateConversation(context.Context, model.Conversation) error                { return nil }
func (m *mockStore) UpdateConversation(context.Context, string, model.ConversationPatch) error   { return nil }
func (m *mockStore) AppendConversationTurn(context.Context, model.ConversationTurn) error        { return nil }
func (m *mockStore) Close() error                                                                 { return nil }

func TestHandleListSessions(t *testing.T) {
	ms := &mockStore{
		sessions: []model.Session{
			{ID: "s1", Provider: "claude-code", Status: "completed", StartedAt: time.Now()},
			{ID: "s2", Provider: "codex", Status: "running", StartedAt: time.Now()},
		},
	}

	srv, err := NewServer(Config{Port: 0}, ms)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/sessions?limit=10", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListResponse[model.Session]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)
	assert.Len(t, resp.Data, 2)
}

func TestHandleGetSession_NotFound(t *testing.T) {
	ms := &mockStore{}
	srv, err := NewServer(Config{Port: 0}, ms)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/sessions/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleListConversations(t *testing.T) {
	ms := &mockStore{
		conversations: []model.Conversation{
			{ID: "c1", Status: "running", CreatedAt: time.Now(), MaxTurns: 10},
		},
	}

	srv, err := NewServer(Config{Port: 0}, ms)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/conversations?limit=10", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListResponse[model.Conversation]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)
}

func TestHandleGetConversationTurns(t *testing.T) {
	ms := &mockStore{
		conversations: []model.Conversation{
			{ID: "c1", Status: "running", CreatedAt: time.Now(), MaxTurns: 10},
		},
		convTurns: map[string][]model.ConversationTurn{
			"c1": {
				{ConversationID: "c1", Index: 0, From: "a", Response: "hello"},
				{ConversationID: "c1", Index: 1, From: "b", Response: "hi"},
			},
		},
	}

	srv, err := NewServer(Config{Port: 0}, ms)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/conversations/c1/turns?limit=10", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListResponse[model.ConversationTurn]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 2)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/api/... -run "TestHandle" -v`
Expected: FAIL — `ListResponse`, handler functions not defined

- [ ] **Step 3: Implement REST handlers**

```go
// internal/api/handlers_rest.go
package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// ListResponse is the paginated response envelope.
type ListResponse[T any] struct {
	Data   []T `json:"data"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	filter := model.SessionFilter{
		Limit:  parseIntParam(r, "limit", 20),
		Offset: parseIntParam(r, "offset", 0),
	}
	if status := r.URL.Query().Get("status"); status != "" {
		rs := model.RunStatus(status)
		filter.Status = &rs
	}

	sessions, total, err := s.store.ListSessions(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if sessions == nil {
		sessions = []model.Session{}
	}
	writeJSON(w, http.StatusOK, ListResponse[model.Session]{
		Data: sessions, Total: total, Limit: filter.Limit, Offset: filter.Offset,
	})
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r.URL.Path, "/api/v1/sessions/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "missing session ID")
		return
	}

	sess, err := s.store.GetSession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if sess == nil {
		writeError(w, http.StatusNotFound, "not_found", "session "+id+" does not exist")
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleSessionTurns(w http.ResponseWriter, r *http.Request) {
	// Path: /api/v1/sessions/{id}/turns
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/"), "/")
	if len(parts) < 2 || parts[1] != "turns" {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid path")
		return
	}
	id := parts[0]
	limit := parseIntParam(r, "limit", 50)

	turns, err := s.store.PeekTurns(r.Context(), id, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if turns == nil {
		turns = []model.Turn{}
	}
	writeJSON(w, http.StatusOK, ListResponse[model.Turn]{
		Data: turns, Total: len(turns), Limit: limit, Offset: 0,
	})
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	filter := model.ConversationFilter{
		Limit:  parseIntParam(r, "limit", 20),
		Offset: parseIntParam(r, "offset", 0),
	}
	if status := r.URL.Query().Get("status"); status != "" {
		cs := model.ConversationStatus(status)
		filter.Status = &cs
	}

	convs, total, err := s.store.ListConversations(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if convs == nil {
		convs = []model.Conversation{}
	}
	writeJSON(w, http.StatusOK, ListResponse[model.Conversation]{
		Data: convs, Total: total, Limit: filter.Limit, Offset: filter.Offset,
	})
}

func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r.URL.Path, "/api/v1/conversations/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusBadRequest, "bad_request", "missing conversation ID")
		return
	}

	conv, err := s.store.GetConversation(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if conv == nil {
		writeError(w, http.StatusNotFound, "not_found", "conversation "+id+" does not exist")
		return
	}
	writeJSON(w, http.StatusOK, conv)
}

func (s *Server) handleConversationTurns(w http.ResponseWriter, r *http.Request) {
	// Path: /api/v1/conversations/{id}/turns
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/conversations/"), "/")
	if len(parts) < 2 || parts[1] != "turns" {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid path")
		return
	}
	id := parts[0]
	limit := parseIntParam(r, "limit", 50)

	turns, err := s.store.PeekConversationTurns(r.Context(), id, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if turns == nil {
		turns = []model.ConversationTurn{}
	}
	writeJSON(w, http.StatusOK, ListResponse[model.ConversationTurn]{
		Data: turns, Total: len(turns), Limit: limit, Offset: 0,
	})
}

func parseIntParam(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return defaultVal
	}
	return v
}

func extractPathParam(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	return s
}
```

- [ ] **Step 4: Register REST routes in server.go**

Update `registerRoutes()` in `internal/api/server.go`:

```go
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/v1/status", s.handleStatus)
	apiMux.HandleFunc("GET /api/v1/sessions", s.handleListSessions)
	apiMux.HandleFunc("GET /api/v1/sessions/{id}", s.handleGetSession)
	apiMux.HandleFunc("GET /api/v1/sessions/{id}/turns", s.handleSessionTurns)
	apiMux.HandleFunc("GET /api/v1/conversations", s.handleListConversations)
	apiMux.HandleFunc("GET /api/v1/conversations/{id}", s.handleGetConversation)
	apiMux.HandleFunc("GET /api/v1/conversations/{id}/turns", s.handleConversationTurns)

	var handler http.Handler = apiMux
	handler = apiKeyMiddleware(s.cfg.APIKey, handler)
	handler = corsMiddleware(s.cfg.CORSOrigin, handler)
	s.mux.Handle("/api/", handler)
}
```

Note: Go 1.22+ `http.ServeMux` supports `{id}` path parameters. Update the handlers to use `r.PathValue("id")` instead of manual path parsing. Replace `extractPathParam` calls:

In `handleGetSession`: replace `id := extractPathParam(...)` with `id := r.PathValue("id")`
In `handleGetConversation`: replace `id := extractPathParam(...)` with `id := r.PathValue("id")`
In `handleSessionTurns`: replace path splitting with `id := r.PathValue("id")`
In `handleConversationTurns`: replace path splitting with `id := r.PathValue("id")`

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/api/... -run "TestHandle" -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis
git add internal/api/handlers_rest.go internal/api/handlers_rest_test.go internal/api/server.go
git commit -m "feat(api): add REST handlers for sessions and conversations"
```

---

### Task 7: Filesystem Watcher for Live Events

**Files:**
- Create: `internal/api/watcher.go`
- Create: `internal/api/watcher_test.go`

- [ ] **Step 1: Add fsnotify dependency**

Run: `cd /home/kamil-rybacki/Code/vitis && go get github.com/fsnotify/fsnotify`

- [ ] **Step 2: Write the failing tests**

```go
// internal/api/watcher_test.go
package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcher_DetectsNewConversation(t *testing.T) {
	dir := t.TempDir()
	convDir := filepath.Join(dir, "conversations")
	require.NoError(t, os.MkdirAll(convDir, 0o700))

	w, err := NewWatcher(dir)
	require.NoError(t, err)
	defer w.Close()

	events := w.Events()

	// Create a conversation directory and metadata file
	cDir := filepath.Join(convDir, "conv-test")
	require.NoError(t, os.MkdirAll(cDir, 0o700))
	conv := model.Conversation{ID: "conv-test", Status: "running"}
	data, _ := json.Marshal(conv)
	require.NoError(t, os.WriteFile(filepath.Join(cDir, "conversation.json"), data, 0o600))

	select {
	case evt := <-events:
		assert.Equal(t, WatchEventConversationCreated, evt.Kind)
		assert.Equal(t, "conv-test", evt.ConversationID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestWatcher_DetectsNewTurn(t *testing.T) {
	dir := t.TempDir()
	convDir := filepath.Join(dir, "conversations", "conv-turn")
	require.NoError(t, os.MkdirAll(convDir, 0o700))

	// Pre-create conversation metadata so the watcher watches this dir
	conv := model.Conversation{ID: "conv-turn", Status: "running"}
	data, _ := json.Marshal(conv)
	require.NoError(t, os.WriteFile(filepath.Join(convDir, "conversation.json"), data, 0o600))

	w, err := NewWatcher(dir)
	require.NoError(t, err)
	defer w.Close()

	events := w.Events()

	// Write a turn file
	turn := model.ConversationTurn{ConversationID: "conv-turn", Index: 0, From: "a", Response: "hello"}
	turnData, _ := json.Marshal(turn)
	turnLine := append(turnData, '\n')
	require.NoError(t, os.WriteFile(filepath.Join(convDir, "turns.jsonl"), turnLine, 0o600))

	select {
	case evt := <-events:
		assert.Equal(t, WatchEventTurnAppended, evt.Kind)
		assert.Equal(t, "conv-turn", evt.ConversationID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/api/... -run "TestWatcher" -v`
Expected: FAIL — `NewWatcher`, `WatchEventConversationCreated` not defined

- [ ] **Step 4: Implement watcher**

```go
// internal/api/watcher.go
package api

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// WatchEventKind describes what changed in the store directory.
type WatchEventKind string

const (
	WatchEventConversationCreated WatchEventKind = "conversation_created"
	WatchEventConversationUpdated WatchEventKind = "conversation_updated"
	WatchEventTurnAppended        WatchEventKind = "turn_appended"
)

// WatchEvent is a file-system change mapped to a Vitis domain event.
type WatchEvent struct {
	Kind           WatchEventKind
	ConversationID string
	FilePath       string
}

// Watcher monitors the store directory for file changes.
type Watcher struct {
	fsw    *fsnotify.Watcher
	root   string
	events chan WatchEvent
	done   chan struct{}
}

// NewWatcher creates a watcher on the store root directory.
func NewWatcher(root string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fsw:    fsw,
		root:   root,
		events: make(chan WatchEvent, 64),
		done:   make(chan struct{}),
	}

	convDir := filepath.Join(root, "conversations")
	if err := os.MkdirAll(convDir, 0o700); err != nil {
		fsw.Close()
		return nil, err
	}
	if err := fsw.Add(convDir); err != nil {
		fsw.Close()
		return nil, err
	}

	// Watch existing conversation subdirectories
	entries, _ := os.ReadDir(convDir)
	for _, e := range entries {
		if e.IsDir() {
			fsw.Add(filepath.Join(convDir, e.Name()))
		}
	}

	go w.loop()
	return w, nil
}

// Events returns the channel of watch events.
func (w *Watcher) Events() <-chan WatchEvent {
	return w.events
}

// Close stops the watcher.
func (w *Watcher) Close() error {
	err := w.fsw.Close()
	<-w.done
	return err
}

func (w *Watcher) loop() {
	defer close(w.done)
	for {
		select {
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handleFSEvent(event)
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) handleFSEvent(event fsnotify.Event) {
	if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
		return
	}

	relPath, err := filepath.Rel(w.root, event.Name)
	if err != nil {
		return
	}
	parts := strings.Split(filepath.ToSlash(relPath), "/")

	// conversations/<id>/conversation.json
	if len(parts) == 3 && parts[0] == "conversations" && parts[2] == "conversation.json" {
		convID := parts[1]
		// Watch the new conversation directory for future turn writes
		convDir := filepath.Dir(event.Name)
		w.fsw.Add(convDir)

		kind := WatchEventConversationCreated
		if event.Op&fsnotify.Write != 0 {
			kind = WatchEventConversationUpdated
		}
		w.events <- WatchEvent{Kind: kind, ConversationID: convID, FilePath: event.Name}
		return
	}

	// conversations/<id>/turns.jsonl
	if len(parts) == 3 && parts[0] == "conversations" && parts[2] == "turns.jsonl" {
		w.events <- WatchEvent{
			Kind:           WatchEventTurnAppended,
			ConversationID: parts[1],
			FilePath:       event.Name,
		}
		return
	}

	// New conversation directory created — start watching it
	if len(parts) == 2 && parts[0] == "conversations" {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			w.fsw.Add(event.Name)
		}
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/api/... -run "TestWatcher" -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis
git add internal/api/watcher.go internal/api/watcher_test.go go.mod go.sum
git commit -m "feat(api): add fsnotify-based file watcher for live conversation events"
```

---

### Task 8: SSE Handlers

**Files:**
- Create: `internal/api/handlers_sse.go`
- Create: `internal/api/handlers_sse_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/api/handlers_sse_test.go
package api

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSE_ConversationStream_SendsTurnEvents(t *testing.T) {
	dir := t.TempDir()
	convDir := filepath.Join(dir, "conversations", "conv-sse")
	require.NoError(t, os.MkdirAll(convDir, 0o700))

	// Pre-create conversation
	conv := model.Conversation{ID: "conv-sse", Status: "running"}
	data, _ := json.Marshal(conv)
	require.NoError(t, os.WriteFile(filepath.Join(convDir, "conversation.json"), data, 0o600))

	ms := &mockStore{
		conversations: []model.Conversation{conv},
		convTurns:     map[string][]model.ConversationTurn{},
	}

	watcher, err := NewWatcher(dir)
	require.NoError(t, err)
	defer watcher.Close()

	srv, err := NewServer(Config{Port: 0}, ms)
	require.NoError(t, err)
	srv.watcher = watcher
	srv.registerRoutes()

	ts := httptest.NewServer(srv.mux)
	defer ts.Close()

	// Start SSE connection in background
	resp, err := http.Get(ts.URL + "/api/v1/conversations/conv-sse/stream")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	// Write a turn to trigger watcher
	turn := model.ConversationTurn{ConversationID: "conv-sse", Index: 0, From: "a", Response: "hello"}
	turnData, _ := json.Marshal(turn)
	require.NoError(t, os.WriteFile(filepath.Join(convDir, "turns.jsonl"), append(turnData, '\n'), 0o600))

	// Read SSE event
	scanner := bufio.NewScanner(resp.Body)
	var eventType, eventData string
	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for SSE event")
		default:
		}
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			eventData = strings.TrimPrefix(line, "data: ")
			break
		}
	}

	assert.Equal(t, "turn", eventType)
	assert.Contains(t, eventData, "conv-sse")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/api/... -run "TestSSE" -v`
Expected: FAIL — `srv.watcher` field, SSE handler not defined

- [ ] **Step 3: Add watcher field to Server**

In `internal/api/server.go`, add `watcher *Watcher` field to the `Server` struct, and update `NewServer` to accept it:

```go
type Server struct {
	cfg       Config
	store     store.Store
	watcher   *Watcher
	mux       *http.ServeMux
	listener  net.Listener
	startedAt time.Time
	sseCount  atomic.Int64
}
```

- [ ] **Step 4: Implement SSE handlers**

```go
// internal/api/handlers_sse.go
package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

const maxSSEConnections = 10

func (s *Server) handleConversationStream(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if convID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "missing conversation ID")
		return
	}

	if s.watcher == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "live events not available")
		return
	}

	if s.sseCount.Load() >= maxSSEConnections {
		writeError(w, http.StatusTooManyRequests, "too_many_streams", "max concurrent SSE connections reached")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal", "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	s.sseCount.Add(1)
	defer s.sseCount.Add(-1)

	events := s.watcher.Events()
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-events:
			if !ok {
				return
			}
			if evt.ConversationID != convID {
				continue
			}
			switch evt.Kind {
			case WatchEventTurnAppended:
				s.sendTurnEvents(w, flusher, evt)
			case WatchEventConversationCreated, WatchEventConversationUpdated:
				s.sendControlEvent(w, flusher, evt)
			}
		}
	}
}

func (s *Server) handleConversationsLifecycleStream(w http.ResponseWriter, r *http.Request) {
	if s.watcher == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "live events not available")
		return
	}

	if s.sseCount.Load() >= maxSSEConnections {
		writeError(w, http.StatusTooManyRequests, "too_many_streams", "max concurrent SSE connections reached")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal", "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	s.sseCount.Add(1)
	defer s.sseCount.Add(-1)

	events := s.watcher.Events()
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-events:
			if !ok {
				return
			}
			if evt.Kind == WatchEventConversationCreated || evt.Kind == WatchEventConversationUpdated {
				s.sendLifecycleEvent(w, flusher, evt)
			}
		}
	}
}

func (s *Server) sendTurnEvents(w http.ResponseWriter, flusher http.Flusher, evt WatchEvent) {
	file, err := os.Open(evt.FilePath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		fmt.Fprintf(w, "event: turn\ndata: %s\n\n", scanner.Text())
		flusher.Flush()
	}
}

func (s *Server) sendControlEvent(w http.ResponseWriter, flusher http.Flusher, evt WatchEvent) {
	data, err := os.ReadFile(evt.FilePath)
	if err != nil {
		return
	}
	eventType := "conversation_updated"
	if evt.Kind == WatchEventConversationCreated {
		eventType = "conversation_started"
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(data))
	flusher.Flush()
}

func (s *Server) sendLifecycleEvent(w http.ResponseWriter, flusher http.Flusher, evt WatchEvent) {
	data, err := os.ReadFile(evt.FilePath)
	if err != nil {
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}

	eventType := "status_changed"
	if evt.Kind == WatchEventConversationCreated {
		eventType = "conversation_started"
	}
	out, _ := json.Marshal(payload)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(out))
	flusher.Flush()
}
```

- [ ] **Step 5: Register SSE routes**

Update `registerRoutes()` in `internal/api/server.go` to add:

```go
	apiMux.HandleFunc("GET /api/v1/conversations/{id}/stream", s.handleConversationStream)
	apiMux.HandleFunc("GET /api/v1/conversations/stream", s.handleConversationsLifecycleStream)
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/api/... -run "TestSSE" -v -timeout 10s`
Expected: PASS

- [ ] **Step 7: Run full api test suite**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/api/... -v -timeout 30s`
Expected: all PASS

- [ ] **Step 8: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis
git add internal/api/handlers_sse.go internal/api/handlers_sse_test.go internal/api/server.go
git commit -m "feat(api): add SSE handlers for live conversation event streaming"
```

---

### Task 9: `vitis serve` CLI Command

**Files:**
- Create: `internal/cli/serve.go`
- Create: `internal/cli/serve_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/serve_test.go
package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServeCommand_ParsesFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so serve exits right away

	code := ServeCommand(ctx, []string{
		"--port", "9999",
		"--log-path", t.TempDir(),
	}, &stdout, &stderr)

	// Exit code 0 because context was cancelled (graceful shutdown)
	assert.Equal(t, 0, code)
}

func TestServeCommand_InvalidPort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ctx := context.Background()

	code := ServeCommand(ctx, []string{
		"--port", "-1",
		"--log-path", t.TempDir(),
	}, &stdout, &stderr)

	assert.NotEqual(t, 0, code)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/cli/... -run "TestServe" -v`
Expected: FAIL — `ServeCommand` not defined

- [ ] **Step 3: Implement ServeCommand**

```go
// internal/cli/serve.go
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/api"
	filestore "github.com/kamilandrzejrybacki-inc/vitis/internal/store/file"
)

// ServeCommand starts the Vitis HTTP event API server.
func ServeCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		port       int
		logPath    string
		apiKey     string
		corsOrigin string
	)
	fs.IntVar(&port, "port", 8090, "HTTP server port")
	fs.StringVar(&logPath, "log-path", "./logs", "file store path")
	fs.StringVar(&apiKey, "api-key", "", "optional API key for /api/v1/ routes")
	fs.StringVar(&corsOrigin, "cors-origin", "", "additional CORS origin (localhost always allowed)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "vitis serve: %v\n", err)
		return 2
	}

	if port < 0 || port > 65535 {
		fmt.Fprintf(stderr, "vitis serve: invalid port %d\n", port)
		return 2
	}

	store, err := filestore.New(logPath, false)
	if err != nil {
		fmt.Fprintf(stderr, "vitis serve: store init: %v\n", err)
		return 1
	}
	defer store.Close()

	watcher, err := api.NewWatcher(logPath)
	if err != nil {
		fmt.Fprintf(stderr, "vitis serve: watcher init: %v\n", err)
		return 1
	}
	defer watcher.Close()

	cfg := api.Config{
		Port:       port,
		APIKey:     apiKey,
		CORSOrigin: corsOrigin,
	}

	srv, err := api.NewServer(cfg, store)
	if err != nil {
		fmt.Fprintf(stderr, "vitis serve: server init: %v\n", err)
		return 1
	}
	srv.SetWatcher(watcher)

	fmt.Fprintf(stderr, "vitis serve: listening on %s\n", srv.Addr())

	if err := srv.ListenAndServe(ctx); err != nil {
		fmt.Fprintf(stderr, "vitis serve: %v\n", err)
		return 1
	}
	return 0
}
```

- [ ] **Step 4: Add SetWatcher method to Server**

In `internal/api/server.go`, add:

```go
// SetWatcher attaches a file watcher for live SSE events.
func (s *Server) SetWatcher(w *Watcher) {
	s.watcher = w
	s.registerRoutes()
}
```

Note: `registerRoutes()` is called again to wire SSE routes that depend on the watcher. Alternatively, restructure so routes are registered in `NewServer` — the watcher is optional and the SSE handlers check `s.watcher == nil` already.

- [ ] **Step 5: Wire ServeCommand into main**

Check how the existing CLI dispatches commands (likely in `main.go` or a root command). Add `"serve"` case that calls `cli.ServeCommand(ctx, args, os.Stdout, os.Stderr)`.

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/cli/... -run "TestServe" -v`
Expected: all PASS

- [ ] **Step 7: Run full test suite**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./... -timeout 60s`
Expected: all PASS

- [ ] **Step 8: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis
git add internal/cli/serve.go internal/cli/serve_test.go internal/api/server.go
git commit -m "feat(cli): add vitis serve command for HTTP event API"
```

---

### Task 10: Integration Smoke Test

**Files:**
- Create: `tests/manual/16_serve_smoke.sh`

- [ ] **Step 1: Create manual integration test**

```bash
#!/usr/bin/env bash
# tests/manual/16_serve_smoke.sh — smoke test for vitis serve
set -euo pipefail
source "$(dirname "$0")/lib/common.sh"

section "vitis serve smoke test"

TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

# Start server in background
go run . serve --port 0 --log-path "$TMPDIR" &
SERVER_PID=$!
sleep 1

# Find the port from stderr (TODO: parse from output)
PORT=8090

step "GET /health"
curl -sf "http://localhost:$PORT/health" | jq .
assert_json_field "http://localhost:$PORT/health" ".status" "ok"

step "GET /api/v1/status"
curl -sf "http://localhost:$PORT/api/v1/status" | jq .

step "GET /api/v1/sessions (empty)"
curl -sf "http://localhost:$PORT/api/v1/sessions?limit=10" | jq .

step "GET /api/v1/conversations (empty)"
curl -sf "http://localhost:$PORT/api/v1/conversations?limit=10" | jq .

kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true

pass "serve smoke test"
```

- [ ] **Step 2: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis
chmod +x tests/manual/16_serve_smoke.sh
git add tests/manual/16_serve_smoke.sh
git commit -m "test(manual): add vitis serve smoke test"
```
