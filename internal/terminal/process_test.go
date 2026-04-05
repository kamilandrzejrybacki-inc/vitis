package terminal

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
)

func collectOutput(ctx context.Context, p PseudoTerminalProcess) []byte {
	var buf bytes.Buffer
	for {
		select {
		case ev, ok := <-p.Output():
			if !ok {
				return buf.Bytes()
			}
			buf.Write(ev.Data)
		case <-ctx.Done():
			return buf.Bytes()
		}
	}
}

func TestProcessSpawnAndCaptureOutput(t *testing.T) {
	rt := NewRuntime()
	spec := adapter.SpawnSpec{
		Command: "echo",
		Args:    []string{"hello world"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := rt.Spawn(spec)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	output := collectOutput(ctx, p)
	combined := string(output)

	if !strings.Contains(combined, "hello world") {
		t.Errorf("expected 'hello world' in output, got: %q", combined)
	}
}

func TestProcessCleanExit(t *testing.T) {
	rt := NewRuntime()
	spec := adapter.SpawnSpec{
		Command: "echo",
		Args:    []string{"clean exit test"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := rt.Spawn(spec)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Drain output so the process can exit cleanly
	go func() {
		for range p.Output() {
		}
	}()

	select {
	case result := <-p.Done():
		if result.Code != 0 {
			t.Errorf("expected exit code 0, got %d (err: %v)", result.Code, result.Err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for process to exit")
	}
}

func TestProcessTerminatePolitely(t *testing.T) {
	rt := NewRuntime()
	spec := adapter.SpawnSpec{
		Command: "sleep",
		Args:    []string{"60"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := rt.Spawn(spec)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Drain output in background
	go func() {
		for range p.Output() {
		}
	}()

	// Give the process a moment to start
	time.Sleep(100 * time.Millisecond)

	if err := p.Terminate(500); err != nil {
		t.Fatalf("Terminate returned error: %v", err)
	}

	deadline := time.After(2 * time.Second)
	select {
	case <-p.Done():
		// process exited as expected
	case <-deadline:
		t.Fatal("process did not exit within 2 seconds after Terminate")
	case <-ctx.Done():
		t.Fatal("test context expired")
	}
}

func TestProcessLargeOutput(t *testing.T) {
	rt := NewRuntime()
	// Use sh -c to run a pipeline that produces 10000 lines
	spec := adapter.SpawnSpec{
		Command: "sh",
		Args:    []string{"-c", "yes | head -n 10000"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := rt.Spawn(spec)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	output := collectOutput(ctx, p)

	if len(output) == 0 {
		t.Error("expected non-empty output from large output test")
	}

	// Verify the output channel closes cleanly by waiting for Done
	select {
	case <-p.Done():
		// channel closed cleanly
	case <-ctx.Done():
		t.Fatal("timed out waiting for large output process to finish")
	}
}
