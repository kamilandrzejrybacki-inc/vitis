package terminal

import (
	"context"
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
)

func TestRuntimeSpawnFailure(t *testing.T) {
	rt := NewRuntime()
	spec := adapter.SpawnSpec{
		Command: "/nonexistent/binary",
	}

	_, err := rt.Spawn(spec)
	if err == nil {
		t.Fatal("expected error when spawning non-existent executable, got nil")
	}
}

func TestRuntimeSpawnWithTerminalDimensions(t *testing.T) {
	tests := []struct {
		name string
		cols int
		rows int
	}{
		{"default dimensions", 0, 0},
		{"explicit dimensions", 120, 40},
		{"large terminal", 220, 50},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rt := NewRuntime()
			spec := adapter.SpawnSpec{
				Command:      "echo",
				Args:         []string{"dimensions test"},
				TerminalCols: tc.cols,
				TerminalRows: tc.rows,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			p, err := rt.Spawn(spec)
			if err != nil {
				t.Fatalf("Spawn() with cols=%d rows=%d returned error: %v", tc.cols, tc.rows, err)
			}

			// Drain output so the process can exit
			go func() {
				for range p.Output() {
				}
			}()

			select {
			case <-p.Done():
				// spawned and exited cleanly
			case <-ctx.Done():
				t.Fatal("timed out waiting for process to exit")
			}
		})
	}
}
