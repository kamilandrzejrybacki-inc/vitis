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
