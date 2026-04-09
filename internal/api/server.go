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

// Config holds HTTP server configuration.
type Config struct {
	Port       int
	APIKey     string
	CORSOrigin string
}

// Server is the Vitis HTTP API server.
type Server struct {
	cfg      Config
	store    store.Store
	mux      *http.ServeMux
	listener net.Listener
	sseCount atomic.Int64
}

// NewServer creates a new Server and binds the listener.
func NewServer(cfg Config, st store.Store) (*Server, error) {
	addr := ":0"
	if cfg.Port != 0 {
		addr = fmt.Sprintf(":%d", cfg.Port)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{
		cfg:      cfg,
		store:    st,
		mux:      http.NewServeMux(),
		listener: ln,
	}
	s.registerRoutes()
	return s, nil
}

// Addr returns the address the server is listening on (e.g. "127.0.0.1:8080").
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// ListenAndServe starts serving and blocks until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // SSE streams need no write timeout
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(s.listener) }()
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/status", s.handleStatus)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"sse_count": s.sseCount.Load(),
	})
}
