package util

import (
	"strings"
	"testing"
)

func TestNewSessionID_Format(t *testing.T) {
	id, err := NewSessionID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(id, "sess_") {
		t.Errorf("expected ID to start with 'sess_', got %q", id)
	}
	// "sess_" (5) + hex of 8 bytes (16) = 21 characters total
	const wantLen = 21
	if len(id) != wantLen {
		t.Errorf("expected length %d, got %d (id=%q)", wantLen, len(id), id)
	}
}

func TestNewID_Format(t *testing.T) {
	id := NewID("conv_")
	if !strings.HasPrefix(id, "conv_") {
		t.Errorf("expected prefix 'conv_', got %q", id)
	}
	if len(id) != len("conv_")+16 {
		t.Errorf("expected length %d, got %d (id=%q)", len("conv_")+16, len(id), id)
	}
}

func TestNewID_EmptyPrefix(t *testing.T) {
	id := NewID("")
	if len(id) != 16 {
		t.Errorf("expected 16 hex chars, got %d (%q)", len(id), id)
	}
}

func TestNewID_Unique(t *testing.T) {
	seen := make(map[string]struct{}, 200)
	for i := 0; i < 200; i++ {
		id := NewID("x_")
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id at iteration %d: %q", i, id)
		}
		seen[id] = struct{}{}
	}
}

func TestLookPath_Found(t *testing.T) {
	// "go" is guaranteed to be on PATH because we're running tests with it.
	if _, err := LookPath("go"); err != nil {
		t.Errorf("expected to find 'go' on PATH: %v", err)
	}
}

func TestLookPath_NotFound(t *testing.T) {
	if _, err := LookPath("vitis-totally-nonexistent-binary-zzz"); err == nil {
		t.Error("expected error for nonexistent binary")
	}
}

func TestNewSessionID_Unique(t *testing.T) {
	const n = 100
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id, err := NewSessionID()
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID generated: %q", id)
		}
		seen[id] = struct{}{}
	}
}
