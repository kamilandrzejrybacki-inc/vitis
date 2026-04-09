package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestServeCommand_ParsesFlags(t *testing.T) {
	// Port 0 lets the OS assign a free port; we cancel immediately after the
	// server announces its address so the test is fast and deterministic.
	ctx, cancel := context.WithCancel(context.Background())

	dir := t.TempDir()
	var stdout, stderr safeBuffer

	done := make(chan int, 1)
	go func() {
		done <- ServeCommand(ctx, []string{
			"--port", "0",
			"--log-path", dir,
			"--api-key", "testkey",
			"--cors-origin", "http://localhost:3000",
		}, &stdout, &stderr)
	}()

	// Wait until the server has printed its listening address.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(stdout.String(), "listening on") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("ServeCommand returned %d; stderr: %s", code, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ServeCommand did not exit after context cancellation")
	}

	out := stdout.String()
	if !strings.Contains(out, "listening on") {
		t.Fatalf("expected 'listening on' in stdout, got: %s", out)
	}
}

func TestServeCommand_StartsServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dir := t.TempDir()
	var stdout, stderr safeBuffer

	done := make(chan int, 1)
	go func() {
		done <- ServeCommand(ctx, []string{
			"--port", "0",
			"--log-path", dir,
		}, &stdout, &stderr)
	}()

	// Wait until the server has printed its listening address.
	var addr string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		out := stdout.String()
		if idx := strings.Index(out, "listening on "); idx >= 0 {
			rest := out[idx+len("listening on "):]
			// Trim any trailing newline.
			addr = strings.TrimSpace(rest)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if addr == "" {
		t.Fatalf("server did not announce address within timeout; stdout: %s stderr: %s", stdout.String(), stderr.String())
	}

	// Hit the health endpoint.
	url := fmt.Sprintf("http://%s/health", addr)
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /health: expected 200, got %d", resp.StatusCode)
	}

	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("ServeCommand exited with code %d; stderr: %s", code, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ServeCommand did not exit after context cancellation")
	}
}

func TestServeCommand_RejectsInvalidPort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ServeCommand(context.Background(), []string{
		"--port", "99999",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 for invalid port, got %d", code)
	}
	if !strings.Contains(stderr.String(), "port") {
		t.Fatalf("expected 'port' in stderr, got: %s", stderr.String())
	}
}
